#!/usr/bin/env python3

import time
import os
import json
import click
import pandas as pd
import numpy as np
import traceback
from typing import Optional, List, Dict, Any, Union, Set
from rich.console import Console
from rich.panel import Panel
from rich.markdown import Markdown
from rich.table import Table
import psycopg2
from psycopg2.extras import execute_values
import dspy
from dotenv import load_dotenv
from datetime import datetime
from pathlib import Path
import re

# Load environment variables
load_dotenv()

console = Console()

#######################
# Helper Functions    #
#######################


def safe_get_list(obj: Any, default: List = None) -> List:
    """
    Safely convert various data structures to a Python list.

    Args:
        obj: Object to convert to a list (numpy array, list, scalar, etc.)
        default: Default value to return if conversion fails

    Returns:
        Python list representation of the object
    """
    if default is None:
        default = []

    if obj is None:
        return default

    try:
        # If it's already a list
        if isinstance(obj, list):
            return obj

        # If it's a numpy array
        if hasattr(obj, "dtype") and hasattr(obj, "tolist"):
            return obj.tolist()

        # If it's a scalar value
        if isinstance(obj, (str, int, float)):
            return [obj]

        # Otherwise try to convert to list
        return list(obj)
    except Exception:
        return default


def is_empty_or_none(obj: Any) -> bool:
    """
    Check if an object is None, empty, or contains only NaN values.
    Works with scalar values, lists, numpy arrays, etc.

    Args:
        obj: Object to check

    Returns:
        True if the object is None or empty, False otherwise
    """
    if obj is None:
        return True

    # Empty collections
    if hasattr(obj, "__len__") and len(obj) == 0:
        return True

    # For numpy arrays with 0 size
    if hasattr(obj, "size") and obj.size == 0:
        return True

    # For pandas Series/DataFrames
    if hasattr(obj, "empty") and obj.empty:
        return True

    # Try to catch other empties
    try:
        if not obj:
            return True
    except (TypeError, ValueError):
        pass

    return False


def clean_metadata_from_text(text: str) -> str:
    """
    Clean metadata header from text if present.

    Args:
        text: Text to clean

    Returns:
        Cleaned text with metadata header removed
    """
    if not isinstance(text, str):
        return text

    # Check if text starts with metadata pattern
    metadata_pattern = r"^source: [^.]+\.\s*author: [^.]+\.\s*channel: [^.]+\.\s*created_at: [^.]+\.\s*is_my_message: (?:True|False)\.\s*"
    return re.sub(metadata_pattern, "", text)


def safe_str(obj: Any, default: str = "") -> str:
    """
    Safely convert an object to string.

    Args:
        obj: Object to convert
        default: Default value if conversion fails

    Returns:
        String representation or default
    """
    if pd.isna(obj) or obj is None:
        return default
    try:
        return str(obj)
    except Exception:
        return default


# Set up DSPy
def init_dspy():
    """Configure DSPy with default model (openai/gpt-4o)"""
    try:
        # Set up DSPy with default model
        lm = dspy.LM(model="openai/gpt-4o")
        dspy.configure(lm=lm)
        return True
    except Exception as e:
        # Check if environment variable is set to skip dspy
        if not os.getenv("SKIP_DSPY"):
            console.print(f"[yellow]Warning: Could not initialize DSPy: {e}[/yellow]")
            console.print(
                "[yellow]Set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variables.[/yellow]"
            )
        return False


# Set up MLflow
def init_mlflow():
    try:
        import mlflow

        mlflow.set_experiment("SimpleMemory")
        mlflow.dspy.autolog()

        console.print("[bold blue] Logging to MLflow...")
    except Exception as e:
        console.print(f"[yellow]Error initializing MLflow: {e}[/yellow]")


def ensure_database(
    db_name: str, db_user: str, db_password: str, db_host: str, db_port: str
):
    """
    Ensure the PostgreSQL database exists, create it if it doesn't.
    Returns a connection to the database.
    """
    # First connect to default database to check if our target database exists
    try:
        conn = psycopg2.connect(
            user=db_user,
            password=db_password,
            host=db_host,
            port=db_port,
            database="postgres",
        )
        conn.autocommit = True
        cursor = conn.cursor()

        # Check if database exists
        cursor.execute(f"SELECT 1 FROM pg_database WHERE datname = '{db_name}'")
        exists = cursor.fetchone()

        if not exists:
            console.print(f"Creating database '{db_name}'...")
            cursor.execute(f"CREATE DATABASE {db_name}")
            console.print(f"Database '{db_name}' created successfully.")

        # Close connection to postgres database
        cursor.close()
        conn.close()

        # Connect to the target database
        conn = psycopg2.connect(
            user=db_user,
            password=db_password,
            host=db_host,
            port=db_port,
            database=db_name,
        )
        return conn

    except Exception as e:
        console.print(f"[red]Error ensuring database: {e}[/red]")
        raise


def create_tables(conn, recreate=False):
    """Create PostgreSQL tables for the SimpleMemory schema"""
    cursor = conn.cursor()
    try:
        # First create the hstore extension if it doesn't exist
        cursor.execute("CREATE EXTENSION IF NOT EXISTS hstore")
        # Create extension for case-insensitive text if it doesn't exist
        cursor.execute("CREATE EXTENSION IF NOT EXISTS citext")

        # Drop tables if recreate is True
        if recreate:
            console.print("[yellow]Dropping existing tables...[/yellow]")
            # Need to drop text_units first due to foreign key constraint
            cursor.execute("DROP TABLE IF EXISTS text_units")
            cursor.execute("DROP TABLE IF EXISTS documents")
            cursor.execute("DROP VIEW IF EXISTS text_units_with_doc")

        # Create documents table
        cursor.execute(
            """
        CREATE TABLE IF NOT EXISTS documents (
            id TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            source CITEXT,
            author CITEXT,
            channel CITEXT,
            created_at TIMESTAMP,
            tags CITEXT[],
            metadata HSTORE
        )
        """
        )

        # Create text_units (chunks) table
        cursor.execute(
            """
        CREATE TABLE IF NOT EXISTS text_units (
            id TEXT PRIMARY KEY,
            text TEXT NOT NULL,
            n_tokens INTEGER,
            document_id TEXT NOT NULL,
            tags CITEXT[],  -- Case-insensitive text array
            FOREIGN KEY (document_id) REFERENCES documents(id)
        )
        """
        )

        # Create indices for better performance
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_text_units_document_id ON text_units(document_id)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_text_units_tags ON text_units USING GIN (tags)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_documents_tags ON documents USING GIN (tags)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_documents_metadata ON documents USING GIN (metadata)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_text_units_text ON text_units USING GIN (to_tsvector('english', text))"
        )
        # Add index on source, author, channel for filtering
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_documents_author ON documents(author)"
        )
        cursor.execute(
            "CREATE INDEX IF NOT EXISTS idx_documents_channel ON documents(channel)"
        )

        # Create view for full-text search over text units
        cursor.execute(
            """
        CREATE OR REPLACE VIEW text_units_with_doc AS
        SELECT 
            tu.id,
            tu.text,
            tu.n_tokens,
            tu.document_id,
            tu.tags,
            d.title as document_title,
            d.source as document_source,
            d.author as document_author,
            d.channel as document_channel,
            d.created_at as document_created_at,
            d.tags as document_tags,
            d.metadata as document_metadata
        FROM 
            text_units tu
        JOIN 
            documents d ON tu.document_id = d.id
        """
        )

        conn.commit()
        console.print("[green]Tables created successfully.[/green]")

    except Exception as e:
        conn.rollback()
        console.print(f"[red]Error creating tables: {e}[/red]")
        raise
    finally:
        cursor.close()


def batched_import(conn, table_name, df, batch_size=100):
    """
    Import a dataframe into PostgreSQL using a batched approach with a progress bar.
    """
    if df.empty:
        console.print(f"[yellow]No data to import for {table_name}[/yellow]")
        return 0

    total = len(df)
    start_s = time.time()

    # Get column names from the dataframe
    columns = df.columns.tolist()

    cursor = conn.cursor()
    try:
        # Use rich progress for nicer output
        from rich.progress import Progress, TextColumn, BarColumn, SpinnerColumn
        from rich.progress import TimeElapsedColumn, TimeRemainingColumn

        # Create progress bar with spinner, percentage, and time stats
        with Progress(
            SpinnerColumn(),
            TextColumn("[bold blue]{task.description}"),
            BarColumn(),
            TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
            TextColumn("[cyan]{task.completed}/{task.total}"),
            TimeElapsedColumn(),
            TimeRemainingColumn(),
            console=console,
        ) as progress:
            # Add a task for tracking progress
            task = progress.add_task(f"Importing {table_name}", total=total)

            for start in range(0, total, batch_size):
                end = min(start + batch_size, total)
                batch = df.iloc[start:end]

                # Prepare values as a list of tuples
                values = [tuple(row) for row in batch.values]

                # Insert batch with updates on conflict
                excluded_columns = ", ".join(
                    [f"{col} = EXCLUDED.{col}" for col in columns if col != "id"]
                )
                query = f"INSERT INTO {table_name} ({', '.join(columns)}) VALUES %s ON CONFLICT (id) DO UPDATE SET {excluded_columns}"
                execute_values(cursor, query, values, template=None)

                # Commit after each batch
                conn.commit()

                # Update progress
                progress.update(task, completed=end)

        elapsed = time.time() - start_s
        rows_per_sec = total / elapsed if elapsed > 0 else 0
        console.print(
            f"[green]{total} rows imported into {table_name} in {elapsed:.2f}s ({rows_per_sec:.0f} rows/sec).[/green]"
        )
        return total

    except Exception as e:
        conn.rollback()
        console.print(f"[red]Error importing data to {table_name}: {e}[/red]")
        raise
    finally:
        cursor.close()


def import_documents(conn, graphrag_folder):
    """Import documents from GraphRAG to PostgreSQL"""
    console.print("\nImporting documents...")
    doc_df = pd.read_parquet(
        f"{graphrag_folder}/documents.parquet",
        columns=["id", "title", "creation_date", "metadata"],
    )

    # Process creation_date to proper PostgreSQL datetime format
    if "creation_date" in doc_df.columns:
        # First rename column to created_at
        doc_df = doc_df.rename(columns={"creation_date": "created_at"})

        # Convert timestamps to proper format
        from datetime import datetime
        import dateutil.parser

        def format_timestamp(ts):
            if pd.isna(ts) or ts is None:
                return None

            try:
                # If it's already a pandas Timestamp or datetime
                if isinstance(ts, (pd.Timestamp, datetime)):
                    return ts

                # Try parsing as string to datetime
                try:
                    return dateutil.parser.parse(str(ts))
                except:
                    console.print(f"Warning: Could not parse timestamp: {ts}")
                    return None
            except Exception as e:
                console.print(f"Error formatting timestamp {ts}: {e}")
                return None

        # Apply the conversion
        doc_df["created_at"] = doc_df["created_at"].apply(format_timestamp)

    # Parse metadata if it exists and extract common fields
    if "metadata" in doc_df.columns:
        # Parse metadata string to dictionary
        def parse_metadata(meta_str):
            if not meta_str or pd.isna(meta_str):
                return {}

            # Handle case where metadata is already a dict
            if isinstance(meta_str, dict):
                return meta_str

            # Handle double-encoded strings
            try:
                import json
                import ast

                # Try to parse as JSON first
                try:
                    return json.loads(meta_str)
                except:
                    # Try as Python dict-like string
                    if meta_str.startswith('"') and meta_str.endswith('"'):
                        inner_str = json.loads(meta_str)
                        try:
                            return ast.literal_eval(inner_str)
                        except:
                            cleaned = inner_str.replace("'", '"')
                            return json.loads(cleaned)
                    else:
                        try:
                            return ast.literal_eval(meta_str)
                        except:
                            cleaned = meta_str.replace("'", '"')
                            return json.loads(cleaned)
            except Exception as e:
                console.print(f"Could not parse metadata: {meta_str}, Error: {e}")
                return {}

        # Apply parsing function
        doc_df["parsed_metadata"] = doc_df["metadata"].apply(parse_metadata)

        # Extract common fields from metadata
        def safe_get(obj, key, default=None):
            if isinstance(obj, dict):
                return obj.get(key, default)
            return default

        doc_df["source"] = doc_df["parsed_metadata"].apply(
            lambda x: safe_get(x, "source", "NO_SOURCE")
        )
        doc_df["author"] = doc_df["parsed_metadata"].apply(
            lambda x: (
                safe_get(x, "author", "NO_AUTHOR").upper()
                if safe_get(x, "author")
                else "NO_AUTHOR"
            )
        )
        doc_df["channel"] = doc_df["parsed_metadata"].apply(
            lambda x: safe_get(x, "channel", "NO_CHANNEL")
        )

        # Extract created_at from metadata if available
        def extract_metadata_created_at(row):
            meta = row["parsed_metadata"]
            if isinstance(meta, dict) and "created_at" in meta:
                return format_timestamp(meta["created_at"])
            return row["created_at"]

        doc_df["created_at"] = doc_df.apply(extract_metadata_created_at, axis=1)

        # Convert metadata to HSTORE format
        from psycopg2.extras import HstoreAdapter

        def convert_to_hstore(metadata):
            if not metadata or pd.isna(metadata):
                return None

            # Convert all values to strings
            hstore_dict = {}
            for key, value in metadata.items():
                if value is not None:
                    # Convert non-string values to string
                    if not isinstance(value, str):
                        if isinstance(value, (dict, list)):
                            # For complex objects, convert to JSON string
                            value = json.dumps(value)
                        else:
                            value = str(value)
                    hstore_dict[key] = value

            return hstore_dict

        doc_df["metadata"] = doc_df["parsed_metadata"].apply(convert_to_hstore)

        # Drop the temporary parsed metadata column
        doc_df = doc_df.drop(columns=["parsed_metadata"])

    # Initialize empty tags array for each document
    doc_df["tags"] = None

    # Try to load entities and add them as tags
    try:
        # Check if entities.parquet exists before attempting to read it
        entities_path = f"{graphrag_folder}/entities.parquet"
        if not os.path.exists(entities_path):
            console.print(
                f"[yellow]Warning: Entities file not found at {entities_path}[/yellow]"
            )
            return batched_import(conn, "documents", doc_df)

        entities_df = pd.read_parquet(
            entities_path,
            columns=["id", "title", "type", "text_unit_ids"],
        )

        # Debug entities dataframe structure
        console.print(f"[dim]Entities dataframe shape: {entities_df.shape}[/dim]")
        if entities_df.empty:
            console.print("[yellow]Warning: Entities dataframe is empty[/yellow]")
            return batched_import(conn, "documents", doc_df)

        # Load text unit to document mapping
        text_df = pd.read_parquet(
            f"{graphrag_folder}/text_units.parquet",
            columns=["id", "document_ids"],
        )

        # Convert to dict for faster lookups
        text_to_docs = {}
        for _, row in text_df.iterrows():
            text_unit_id = row["id"]
            if is_empty_or_none(row["document_ids"]):
                continue

            # Get document IDs as a list
            try:
                doc_ids = safe_get_list(row["document_ids"])
                text_to_docs[text_unit_id] = doc_ids
            except Exception as e:
                console.print(
                    f"[yellow]Warning: Could not process document_ids for text unit {text_unit_id}: {e}[/yellow]"
                )
                continue

        # Map entities to documents through text units
        doc_entities = {}
        for _, entity in entities_df.iterrows():
            entity_id = entity["id"]

            # Get entity title and type
            entity_title = safe_str(entity["title"], "unknown").replace('"', "")
            entity_type = safe_str(entity["type"], "entity")

            # Format tag as "entity:type:title"
            tag = f"entity:{entity_type}:{entity_title}"

            # Skip if text_unit_ids is empty
            if is_empty_or_none(entity["text_unit_ids"]):
                continue

            # Get text unit IDs as a list
            try:
                text_unit_ids = safe_get_list(entity["text_unit_ids"])

                # Map to documents
                for text_unit_id in text_unit_ids:
                    if text_unit_id not in text_to_docs:
                        continue

                    doc_ids = text_to_docs.get(text_unit_id, [])
                    for doc_id in doc_ids:
                        if doc_id not in doc_entities:
                            doc_entities[doc_id] = set()
                        doc_entities[doc_id].add(tag)
            except Exception as e:
                console.print(
                    f"[yellow]Warning: Could not process text_unit_ids for entity {entity_id}: {e}[/yellow]"
                )
                continue

        # Add tags to each document - fix the array assignment
        for doc_id, tags in doc_entities.items():
            tags_list = list(tags)
            # Find matching rows
            mask = doc_df["id"] == doc_id
            if any(mask):
                # Assign tags to each matching row one by one
                for idx in doc_df.index[mask]:
                    doc_df.at[idx, "tags"] = tags_list

        console.print(
            f"[green]Added entity tags to {len(doc_entities)} documents[/green]"
        )
    except Exception as e:
        console.print(f"[yellow]Warning: Could not process entities: {e}[/yellow]")
        console.print(f"[dim]{traceback.format_exc()}[/dim]")

    # Add source, channel, and author as tags too using a helper function
    def add_metadata_tags(row):
        # Initialize with existing tags or empty list
        tags = row["tags"] or []

        # Add source tag if valid
        if row["source"] and row["source"] != "NO_SOURCE":
            tags.append(f"source:{row['source']}")

        # Add channel tag if valid
        if row["channel"] and row["channel"] != "NO_CHANNEL":
            tags.append(f"channel:{row['channel']}")

        # Add author tag if valid
        if row["author"] and row["author"] != "NO_AUTHOR":
            tags.append(f"author:{row['author']}")

        return tags

    doc_df["tags"] = doc_df.apply(add_metadata_tags, axis=1)

    # Register psycopg2 adapters for converting Python objects to PostgreSQL
    from psycopg2.extensions import register_adapter
    from psycopg2.extras import register_hstore

    # Enable HSTORE for query results
    with conn.cursor() as cursor:
        register_hstore(cursor)

    # Print document statistics for verification
    total_docs = len(doc_df)
    valid_source = len(doc_df[doc_df["source"] != "NO_SOURCE"])
    valid_author = len(doc_df[doc_df["author"] != "NO_AUTHOR"])
    valid_channel = len(doc_df[doc_df["channel"] != "NO_CHANNEL"])

    console.print(f"Total documents: {total_docs}")
    console.print(
        f"Documents with valid source: {valid_source} ({valid_source/total_docs*100:.1f}%)"
    )
    console.print(
        f"Documents with valid author: {valid_author} ({valid_author/total_docs*100:.1f}%)"
    )
    console.print(
        f"Documents with valid channel: {valid_channel} ({valid_channel/total_docs*100:.1f}%)"
    )

    return batched_import(conn, "documents", doc_df)


def import_text_units(conn, graphrag_folder):
    """Import text units (chunks) from GraphRAG to PostgreSQL"""
    console.print("\nImporting text units (chunks)...")

    text_df = pd.read_parquet(
        f"{graphrag_folder}/text_units.parquet",
        columns=["id", "text", "n_tokens", "document_ids"],
    )

    # Clean metadata from text field if present
    text_df["text"] = text_df["text"].apply(clean_metadata_from_text)

    # Handle the document_ids array - we'll create one row per document ID
    expanded_rows = []
    for _, row in text_df.iterrows():
        # Convert document_ids to list using helper function
        doc_ids = safe_get_list(row["document_ids"])

        if len(doc_ids) > 0:
            for doc_id in doc_ids:
                expanded_rows.append(
                    {
                        "id": row["id"],
                        "text": row["text"],
                        "n_tokens": row["n_tokens"],
                        "document_id": doc_id,
                        "tags": [],  # Initialize empty tags array
                    }
                )

    # Create a new dataframe from the expanded rows
    expanded_df = pd.DataFrame(expanded_rows)

    # If there are duplicate IDs because a text unit belongs to multiple documents,
    # we'll need to create unique IDs
    if len(expanded_df) > len(expanded_df["id"].unique()):
        console.print(
            "[yellow]Warning: Text units belong to multiple documents, creating compound IDs[/yellow]"
        )
        expanded_df["id"] = expanded_df["id"] + "_" + expanded_df["document_id"]

    # Try to load entities associated with text units as tags
    try:
        # Check if entities.parquet exists before attempting to read it
        import os.path

        entities_path = f"{graphrag_folder}/entities.parquet"
        if not os.path.exists(entities_path):
            console.print(
                f"[yellow]Warning: Entities file not found at {entities_path}[/yellow]"
            )
            return batched_import(conn, "text_units", expanded_df)

        entities_df = pd.read_parquet(
            entities_path,
            columns=["id", "title", "type", "text_unit_ids"],
        )

        # Debug entities dataframe structure
        console.print(f"[dim]Entities dataframe shape: {entities_df.shape}[/dim]")
        if entities_df.empty:
            console.print("[yellow]Warning: Entities dataframe is empty[/yellow]")
            return batched_import(conn, "text_units", expanded_df)

        # Create a mapping of text unit IDs to entity tags
        text_unit_entities = {}
        for _, entity in entities_df.iterrows():
            # Get entity title and type using helper functions
            entity_title = safe_str(entity["title"], "unknown").replace('"', "")
            entity_type = safe_str(entity["type"], "entity")

            # Format tag as "entity:type:title"
            tag = f"entity:{entity_type}:{entity_title}"

            # Skip if text_unit_ids is empty
            if is_empty_or_none(entity["text_unit_ids"]):
                continue

            # Handle case where text_unit_ids could be a numpy array
            try:
                # Use the helper function to get text unit IDs as a list
                text_unit_ids = safe_get_list(entity["text_unit_ids"])

                # Debug output for text_unit_ids
                if not text_unit_ids:
                    console.print(
                        f"[yellow]Warning: Entity {entity['id']} has no text units[/yellow]"
                    )

                for text_unit_id in text_unit_ids:
                    if text_unit_id not in text_unit_entities:
                        text_unit_entities[text_unit_id] = set()
                    text_unit_entities[text_unit_id].add(tag)
            except Exception as e:
                console.print(
                    f"[yellow]Warning: Could not process text_unit_ids for entity {entity['id']}: {e}[/yellow]"
                )
                continue

        # Add tags to the expanded dataframe safely
        for idx, row in expanded_df.iterrows():
            # Get the original text unit ID (before compound ID creation)
            original_id = row["id"].split("_")[0] if "_" in row["id"] else row["id"]
            tags = text_unit_entities.get(original_id, set())
            if tags:
                # Convert set to list for storage
                expanded_df.at[idx, "tags"] = list(tags)

        console.print(
            f"[green]Added entity tags to {len(text_unit_entities)} text units[/green]"
        )
    except Exception as e:
        import traceback

        console.print(
            f"[yellow]Warning: Could not process entities for text units: {e}[/yellow]"
        )
        console.print(f"[dim]{traceback.format_exc()}[/dim]")

    return batched_import(conn, "text_units", expanded_df)


###################
# DSPy Signatures #
###################


class ReactStepSignature(dspy.Signature):
    """Generate a reasoning step, action, or answer based on the user query and context."""

    db_schema = dspy.InputField(desc="The PostgreSQL database schema information")
    examples = dspy.InputField(desc="Example queries and their SQL translations")
    question = dspy.InputField(desc="The user's original question")
    past_reasoning = dspy.InputField(
        desc="List of past reasoning steps, each containing thought, query/answer, and observation"
    )

    thought = dspy.OutputField(
        desc="Reasoning about what to do next. This should be a detailed explanation of your reasoning process."
    )
    query = dspy.OutputField(
        desc="Optional SQL query to execute. Provide a valid SQL query string here if you want to query the database. If providing a final answer instead, leave this field empty or set to null."
    )
    answer = dspy.OutputField(
        desc="Answer to the question. Provide the complete answer as a string given the currently available information."
    )


##############
# DSPy Modules
##############


class ReactAgent(dspy.Module):
    """
    ReAct agent for multi-step reasoning with PostgreSQL database.
    Uses a loop of thinking, action, and observation to iteratively solve complex queries.
    """

    def __init__(
        self,
        schema_info: str,
        conn,
        examples: str = None,
        max_steps: int = 5,
        verbose: bool = False,
    ):
        """
        Initialize the ReactAgent for multi-step reasoning.

        Args:
            schema_info: PostgreSQL database schema information
            conn: PostgreSQL database connection
            examples: Example SQL queries to guide the agent
            max_steps: Maximum number of reasoning steps to take
            verbose: Whether to print detailed debugging information
        """
        super().__init__()
        self.schema_info = schema_info
        self.conn = conn
        self.examples = examples if examples else sql_examples()
        self.max_steps = max_steps
        self.verbose = verbose
        self.reasoner = dspy.Predict(ReactStepSignature)

    def forward(self, question: str) -> Dict[str, Any]:
        # Initialize the reasoning loop with an empty list for past reasoning
        past_reasoning = []
        answer = ""
        step = 0

        while step < self.max_steps:
            step += 1

            # Generate next reasoning step
            response = self.reasoner(
                db_schema=self.schema_info,
                examples=self.examples,
                question=question,
                past_reasoning=past_reasoning,
            )

            # Extract and format the thought and actions
            thought = response.thought.strip()
            query = response.query.strip() if response.query else None
            answer = response.answer.strip() if response.answer else None

            # Create a new reasoning step
            current_step = {"step": step, "thought": thought}

            # If we have a query, execute it
            if query and query.lower() != "null" and query.lower() != "none":
                current_step["query"] = query

                # Execute the SQL query
                try:
                    # Clean up the query if it has code blocks
                    sql_query = query
                    if sql_query.startswith("```") and sql_query.endswith("```"):
                        sql_query = sql_query[3:-3].strip()
                    if sql_query.startswith("sql") or sql_query.startswith("SQL"):
                        sql_query = (
                            sql_query.split("\n", 1)[1]
                            if "\n" in sql_query
                            else sql_query[3:].strip()
                        )

                    # Execute query
                    cursor = self.conn.cursor()
                    cursor.execute(sql_query)

                    # Format the results
                    columns = (
                        [desc[0] for desc in cursor.description]
                        if cursor.description
                        else []
                    )
                    records = cursor.fetchall()
                    cursor.close()

                    # Define max results to include in observation
                    max_results = 50

                    # Convert records to dictionaries for easier handling
                    results_data = [
                        dict(zip(columns, record)) for record in records[:max_results]
                    ]
                    results_count = len(records)

                    # Add observation to the current step
                    current_step["observation"] = {
                        "success": True,
                        "results": results_data,
                        "count": results_count,
                        "truncated": results_count > max_results,
                    }

                except Exception as e:
                    # Add error observation to the current step
                    current_step["observation"] = {"success": False, "error": str(e)}

                # Add step to past reasoning
                past_reasoning.append(current_step)
            # If we have an answer, return it
            elif answer:
                current_step["answer"] = answer
                past_reasoning.append(current_step)

                # Format past reasoning for debugging if needed
                if self.verbose:
                    print(json.dumps(past_reasoning, indent=2, cls=CustomJSONEncoder))

                return {
                    "answer": answer,
                    "reason_trace": past_reasoning,
                    "complete": True,
                }

            # Neither query nor answer provided
            else:
                current_step["error"] = "Neither query nor answer was provided"
                past_reasoning.append(current_step)

        return {
            "answer": answer,
            "reason_trace": past_reasoning,
            "complete": False,
        }


#######################
# Database Operations #
#######################


def get_schema_info(conn):
    """Get schema information from PostgreSQL"""
    schema_text = "PostgreSQL Database Schema:\n\n"

    try:
        cursor = conn.cursor()

        # Get tables
        cursor.execute(
            """
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = 'public'
        """
        )
        tables = [row[0] for row in cursor.fetchall()]

        # Get table structure
        for table in tables:
            cursor.execute(
                f"""
            SELECT column_name, data_type, is_nullable
            FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = '{table}'
            ORDER BY ordinal_position
            """
            )
            columns = cursor.fetchall()

            schema_text += f"Table: {table}\n"
            schema_text += "Columns:\n"
            for column_name, data_type, is_nullable in columns:
                nullable = "NULL" if is_nullable == "YES" else "NOT NULL"
                schema_text += f"  - {column_name}: {data_type} {nullable}\n"

            # Get indexes
            cursor.execute(
                f"""
            SELECT indexname, indexdef
            FROM pg_indexes
            WHERE tablename = '{table}'
            """
            )
            indexes = cursor.fetchall()

            if indexes:
                schema_text += "Indexes:\n"
                for index_name, index_def in indexes:
                    schema_text += f"  - {index_name}: {index_def}\n"

            schema_text += "\n"

        # Get views
        cursor.execute(
            """
        SELECT table_name
        FROM information_schema.views
        WHERE table_schema = 'public'
        """
        )
        views = [row[0] for row in cursor.fetchall()]

        if views:
            schema_text += "Views:\n"
            for view in views:
                cursor.execute(
                    f"""
                SELECT pg_get_viewdef('{view}'::regclass, true)
                """
                )
                view_def = cursor.fetchone()[0]
                schema_text += f"  - {view}: {view_def}\n\n"

        cursor.close()

    except Exception as e:
        console.print(f"[red]Error getting schema information: {e}[/red]")
        schema_text += f"Error getting schema information: {e}\n"

    # Add extra schema context that isn't obvious from the DB schema alone
    schema_text += """
Additional Schema Information:

1. Tags are stored as CITEXT arrays for case-insensitive matching
2. Tag format follows these patterns:
   - entity:type:title - for entities extracted from text (ONLY available in text_units table)
   - source:value - for document sources like 'slack' (available in documents' source column)
   - channel:value - for channels like 'engineering' (available in documents' channel column)
   - author:value - for document authors (available in documents' author column)

3. Common entity types are 'PERSON', 'GEO', 'ORGANIZATION', 'EVENT'
4. Document metadata is stored in HSTORE for efficient key-value storage
5. Text units are chunks of documents, with a many-to-one relationship to documents
6. Full-text search can be performed using PostgreSQL's tsquery/tsvector functionality
7. For tag matching:
   - Use @> (contains) operator when you need exact array matching (all elements exactly)
   - Use && (overlap) operator when you need to find any overlap between arrays
   - Always cast arrays to CITEXT[] for case-insensitive matching: ARRAY['tag']::citext[]
   
8. IMPORTANT: To search for entities, always query the text_units table, not documents:
   SELECT tu.id, tu.text, d.title, d.created_at, d.author 
   FROM text_units tu JOIN documents d ON tu.document_id = d.id
   WHERE tu.tags && ARRAY['entity:ORGANIZATION:OpenAI']::citext[]
"""

    return schema_text


def sql_examples():
    """Provide example queries with accurate PostgreSQL schema references"""
    # Define examples as a list of tuples (nl_query, sql_query)
    examples = [
        (
            "Find recent documents from Slack in the 'engineering' channel from the last month",
            """SELECT id, title, created_at, author 
FROM documents 
WHERE source = 'slack' AND channel = 'engineering'
  AND created_at > NOW() - INTERVAL '30 days'
ORDER BY created_at DESC
LIMIT 10""",
        ),
        (
            "Find documents with specific tags",
            """SELECT id, title, created_at, author
FROM documents
WHERE tags && ARRAY['entity:PERSON:John']::citext[]
ORDER BY created_at DESC
LIMIT 10""",
        ),
        (
            "Find text units with OpenAI entity tag (case-insensitive)",
            """SELECT tu.id, tu.text, d.title, d.created_at, d.author
FROM text_units tu
JOIN documents d ON tu.document_id = d.id
WHERE tu.tags && ARRAY['entity:ORGANIZATION:OpenAI']::citext[]
ORDER BY d.created_at DESC
LIMIT 10""",
        ),
        (
            "Find documents by source with case-insensitive matching",
            """SELECT id, title, created_at, author
FROM documents
WHERE source = 'slack'  -- Uses CITEXT for automatic case-insensitive matching
ORDER BY created_at DESC
LIMIT 10""",
        ),
        (
            "Show documents by a specific author with date criteria",
            """SELECT id, title, source, created_at
FROM documents
WHERE author = 'KEN' AND created_at > NOW() - INTERVAL '90 days'
ORDER BY created_at DESC
LIMIT 10""",
        ),
        (
            "Find text units containing specific keywords",
            """SELECT tu.id, tu.text, d.title as document_title, d.author
FROM text_units tu
JOIN documents d ON tu.document_id = d.id
WHERE tu.text ILIKE '%machine learning%' OR tu.text ILIKE '%artificial intelligence%'
LIMIT 10""",
        ),
        (
            "Analyze author communication patterns",
            """SELECT author, channel, COUNT(*) as message_count
FROM documents
WHERE author IS NOT NULL AND author != 'NO_AUTHOR'
  AND channel IS NOT NULL AND channel != 'NO_CHANNEL'
  AND created_at > NOW() - INTERVAL '30 days'
GROUP BY author, channel
ORDER BY message_count DESC
LIMIT 10""",
        ),
        (
            "Find documents with specific metadata",
            """SELECT id, title, metadata->'platform' as platform
FROM documents
WHERE metadata ? 'platform'
LIMIT 10""",
        ),
        (
            "Full-text search in text units",
            """SELECT tu.id, tu.text, d.title, d.author
FROM text_units tu
JOIN documents d ON tu.document_id = d.id
WHERE to_tsvector('english', tu.text) @@ to_tsquery('english', 'machine & learning')
ORDER BY d.created_at DESC
LIMIT 10""",
        ),
    ]

    # Format examples into the expected string format
    formatted_examples = "Example Queries and Translations:\n\n"
    for nl_query, sql_query in examples:
        formatted_examples += (
            f"Natural Language: {nl_query}\nSQL:\n```\n{sql_query}\n```\n\n"
        )

    return formatted_examples


def save_query_history(history_file: Path, entry: dict):
    """Save a query history entry to a JSONL file"""
    try:
        with open(history_file, "a") as f:
            f.write(json.dumps(entry, cls=CustomJSONEncoder) + "\n")
    except Exception as e:
        console.print(f"[yellow]Warning: Failed to save query history: {e}[/yellow]")


# Create a custom JSON encoder to handle non-serializable types
class CustomJSONEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, datetime):
            return obj.isoformat()
        # Handle date objects (not just datetime)
        elif hasattr(obj, "isoformat") and callable(obj.isoformat):
            return obj.isoformat()
        # Handle other custom types like decimal, etc.
        elif hasattr(obj, "__str__"):
            return str(obj)
        return json.JSONEncoder.default(self, obj)


def create_history_entry(query: str, mode: str, **kwargs) -> dict:
    """Create a standard history entry with common fields"""
    entry = {"timestamp": datetime.now().isoformat(), "query": query, "mode": mode}
    # Add any additional fields
    entry.update(kwargs)
    return entry


def process_query(
    conn,
    query: str,
    max_steps=5,
    verbose=False,
    model_name="openai/gpt-4o",
    no_cache=False,
) -> str:
    """Process a natural language query using the ReAct agent

    Args:
        conn: PostgreSQL database connection
        query: Natural language query
        max_steps: Maximum number of reasoning steps
        verbose: Whether to show verbose output
        model_name: Model to use (e.g., "openai/gpt-4o", "anthropic/claude-3-opus-20240229")
        no_cache: Whether to disable LLM caching
    """
    start_time = time.time()
    console.print(f"[dim]Using model: {model_name}[/dim]")

    # Get schema information
    schema_info = get_schema_info(conn)

    # Get SQL examples
    examples = sql_examples()

    # Create ReactAgent
    react_agent = ReactAgent(schema_info, conn, examples, max_steps, verbose)

    try:
        # Use the ReAct agent to answer the query
        console.print("\n[bold magenta]Processing query...[/bold magenta]")
        if max_steps > 1:
            console.print(f"(Maximum {max_steps} reasoning steps)")

        # Record time before answer generation
        answer_start_time = time.time()

        # Get the answer using the specified model with context
        lm_params = {"model": model_name}
        if no_cache:
            lm_params["cache"] = None
            if verbose:
                console.print("[dim]Cache disabled[/dim]")

        with dspy.context(lm=dspy.LM(**lm_params)):
            result = react_agent(query)

        # Calculate answer generation time
        answer_time = time.time() - answer_start_time

        # Calculate and display timing information
        total_time = time.time() - start_time
        init_time = answer_start_time - start_time

        console.print(
            f"\n[dim]Timing: init={init_time:.2f}s, answer={answer_time:.2f}s, total={total_time:.2f}s ({total_time/60:.2f}m)[/dim]"
        )

        # Extract data from the result dictionary
        answer = result["answer"]
        reasoning_trace = result["reason_trace"]
        is_complete = result["complete"]

        # Show completion status if verbose
        if verbose:
            completion_status = (
                "Complete" if is_complete else "Incomplete (max steps reached)"
            )
            console.print(f"[dim]Status: {completion_status}[/dim]")

        # Display the answer
        console.print(Panel(Markdown(answer), title="Answer", border_style="magenta"))

        # Save query history if requested
        history_file = Path("query_history.jsonl")
        if answer:
            history_entry = create_history_entry(
                query,
                "react",
                max_steps=max_steps,
                answer=answer,
                total_seconds=total_time,
                init_seconds=init_time,
                answer_seconds=answer_time,
                model=model_name,
                cache_disabled=no_cache,
                reasoning=(
                    reasoning_trace if verbose else None
                ),  # Only include reasoning trace if verbose
                complete=is_complete,
            )
            save_query_history(history_file, history_entry)

        return answer

    except Exception as e:
        # Calculate timing even for errors
        total_time = time.time() - start_time
        console.print(
            f"\n[dim]Query failed after {total_time:.2f}s ({total_time/60:.2f}m)[/dim]"
        )
        console.print(f"[red]Error processing query: {e}[/red]")
        if verbose:
            import traceback

            console.print(traceback.format_exc())
        return None


@click.group()
@click.option("--db-name", default="simplememory", help="PostgreSQL database name")
@click.option("--db-user", default="postgres", help="PostgreSQL username")
@click.option("--db-password", required=True, help="PostgreSQL password")
@click.option("--db-host", default="localhost", help="PostgreSQL host")
@click.option("--db-port", default="5432", help="PostgreSQL port")
@click.option(
    "--create-db/--no-create-db",
    default=True,
    help="Create database if it doesn't exist",
)
@click.pass_context
def cli(ctx, db_name, db_user, db_password, db_host, db_port, create_db):
    """SimpleMemory - A simpler alternative to GraphRAG using PostgreSQL."""
    # Create a database connection and store it in the context
    ctx.ensure_object(dict)

    # Store database parameters in context
    ctx.obj["db_params"] = {
        "db_name": db_name,
        "db_user": db_user,
        "db_password": db_password,
        "db_host": db_host,
        "db_port": db_port,
    }

    # Connect to database
    try:
        if create_db:
            conn = ensure_database(db_name, db_user, db_password, db_host, db_port)
        else:
            conn = psycopg2.connect(
                user=db_user,
                password=db_password,
                host=db_host,
                port=db_port,
                database=db_name,
            )
        ctx.obj["conn"] = conn
    except Exception as e:
        console.print(f"[red]Error connecting to database: {e}[/red]")
        raise


# The get_db_connection functionality is already covered by ensure_database and the cli function above


@cli.command()
@click.option(
    "--graphrag-folder", required=True, help="Path to GraphRAG artifacts folder"
)
@click.option("--batch-size", default=100, type=int, help="Batch size for imports")
@click.option(
    "--recreate/--no-recreate",
    default=False,
    help="Drop and recreate all tables before import",
)
@click.option(
    "--verbose/--no-verbose", default=False, help="Show detailed schema information"
)
@click.pass_context
def import_graphrag(ctx, graphrag_folder, batch_size, recreate, verbose):
    """Import GraphRAG parquet files into PostgreSQL database."""

    # Get connection from context
    conn = ctx.obj["conn"]

    try:
        # Create tables
        create_tables(conn, recreate=recreate)

        # Import data
        doc_count = import_documents(conn, graphrag_folder)
        text_count = import_text_units(conn, graphrag_folder)

        console.print("\n[bold green]Import complete![/bold green]")
        console.print(f"Documents: {doc_count}")
        console.print(f"Text units: {text_count}")

        # Show tag statistics if verbose
        if verbose:
            cursor = conn.cursor()

            # Count documents with tags
            cursor.execute(
                "SELECT COUNT(*) FROM documents WHERE tags IS NOT NULL AND array_length(tags, 1) > 0"
            )
            docs_with_tags = cursor.fetchone()[0]
            if doc_count > 0:
                console.print(
                    f"Documents with tags: {docs_with_tags} ({docs_with_tags/doc_count*100:.1f}%)"
                )

            # Count text units with tags
            cursor.execute(
                "SELECT COUNT(*) FROM text_units WHERE tags IS NOT NULL AND array_length(tags, 1) > 0"
            )
            texts_with_tags = cursor.fetchone()[0]
            if text_count > 0:
                console.print(
                    f"Text units with tags: {texts_with_tags} ({texts_with_tags/text_count*100:.1f}%)"
                )

            # Show schema information
            schema_info = get_schema_info(conn)
            console.print(
                Panel(schema_info, title="Schema", border_style="blue", expand=False)
            )

            cursor.close()

    except Exception as e:
        console.print(f"[red]Error during import: {e}[/red]")


@cli.command()
@click.option(
    "--recreate/--no-recreate", default=False, help="Drop and recreate all tables"
)
@click.option(
    "--verbose/--no-verbose", default=False, help="Show detailed schema information"
)
@click.pass_context
def setup(ctx, recreate, verbose):
    """Set up the SimpleMemory database schema."""

    # Get connection from context
    conn = ctx.obj["conn"]

    try:
        # Create tables
        create_tables(conn, recreate=recreate)
        console.print("[bold green]Database setup complete![/bold green]")

        # Show schema information if verbose
        if verbose:
            schema_info = get_schema_info(conn)
            console.print(
                Panel(schema_info, title="Schema", border_style="blue", expand=False)
            )

            # Also show table counts
            cursor = conn.cursor()
            cursor.execute("SELECT COUNT(*) FROM documents")
            doc_count = cursor.fetchone()[0]
            cursor.execute("SELECT COUNT(*) FROM text_units")
            text_count = cursor.fetchone()[0]
            cursor.close()

            console.print(f"Documents: {doc_count}")
            console.print(f"Text units: {text_count}")
    except Exception as e:
        console.print(f"[red]Error during setup: {e}[/red]")


@cli.command()
@click.option("--query", required=True, help="SQL query to run")
@click.option("--explain/--no-explain", default=False, help="Show query execution plan")
@click.option(
    "--format",
    type=click.Choice(["table", "json", "csv"]),
    default="table",
    help="Output format (table, json, or csv)",
)
@click.option("--output-file", help="Save results to a file instead of displaying")
@click.pass_context
def query(ctx, query, explain, format, output_file):
    """
    Run a SQL query against the SimpleMemory database.

    Example queries:

    # Find documents from a specific source (case-insensitive):
    SELECT * FROM documents WHERE tags @> ARRAY['source:slack']::citext[] LIMIT 10;

    # Find text units containing specific text:
    SELECT * FROM text_units WHERE text ILIKE '%machine learning%' LIMIT 10;

    # Find documents with specific entities:
    SELECT * FROM documents WHERE tags @> ARRAY['entity:person:john']::citext[] LIMIT 10;

    # Get text units summary with document info:
    SELECT tu.id, LEFT(tu.text, 100) as preview, d.title, d.author
    FROM text_units tu
    JOIN documents d ON tu.document_id = d.id
    LIMIT 10;

    # Query metadata using HSTORE operators:
    SELECT id, title, metadata->'platform' as platform
    FROM documents
    WHERE metadata ? 'platform'
    LIMIT 10;

    # Search for documents with specific metadata value:
    SELECT * FROM documents
    WHERE metadata @> 'platform=>slack'
    LIMIT 10;

    # Combine tags and metadata in a query:
    SELECT d.id, d.title, d.metadata->'channel' as channel
    FROM documents d
    WHERE d.tags @> ARRAY['author:john']::citext[] AND d.metadata ? 'channel'
    LIMIT 10;
    """

    # Get connection from context
    conn = ctx.obj["conn"]

    try:
        cursor = conn.cursor()

        # If explain is requested, run EXPLAIN ANALYZE
        if explain:
            explain_query = f"EXPLAIN ANALYZE {query}"
            cursor.execute(explain_query)
            explain_result = cursor.fetchall()

            console.print("[bold blue]Query Execution Plan:[/bold blue]")
            for row in explain_result:
                console.print(row[0])

            # Execute the actual query after showing the explain plan
            cursor.execute(query)
        else:
            cursor.execute(query)

        try:
            results = cursor.fetchall()
            if not results:
                console.print("[yellow]Query returned no results.[/yellow]")
                return

            # Get column names
            column_names = [desc[0] for desc in cursor.description]

            # Convert results to a list of dictionaries for easier handling
            results_dicts = [dict(zip(column_names, row)) for row in results]

            # Display or save results based on format
            if output_file:
                import csv

                if format == "json":
                    with open(output_file, "w") as f:
                        json.dump(results_dicts, f, default=str, indent=2)
                    console.print(
                        f"[green]Results saved as JSON to {output_file}[/green]"
                    )

                elif format == "csv":
                    with open(output_file, "w", newline="") as f:
                        writer = csv.DictWriter(f, fieldnames=column_names)
                        writer.writeheader()
                        writer.writerows(results_dicts)
                    console.print(
                        f"[green]Results saved as CSV to {output_file}[/green]"
                    )

                else:  # table format isn't really suited for file output, use CSV instead
                    console.print(
                        "[yellow]Table format not supported for file output. Using CSV instead.[/yellow]"
                    )
                    with open(output_file, "w", newline="") as f:
                        writer = csv.DictWriter(f, fieldnames=column_names)
                        writer.writeheader()
                        writer.writerows(results_dicts)
                    console.print(
                        f"[green]Results saved as CSV to {output_file}[/green]"
                    )
            else:
                # Display to console
                console.print(
                    f"[bold green]Query returned {len(results_dicts)} results:[/bold green]"
                )

                if format == "json":
                    console.print_json(json.dumps(results_dicts, default=str))

                elif format == "csv":
                    import io
                    import csv

                    output = io.StringIO()
                    writer = csv.DictWriter(output, fieldnames=column_names)
                    writer.writeheader()
                    writer.writerows(results_dicts)
                    console.print(output.getvalue())

                else:  # table format (default)
                    table = Table(show_header=True, header_style="bold")

                    for col in column_names:
                        table.add_column(col)

                    for row in results:
                        # Special handling for arrays (like tags)
                        row_display = []
                        for cell in row:
                            if isinstance(cell, list):
                                row_display.append(str(cell).replace("'", ""))
                            elif cell is None:
                                row_display.append("NULL")
                            else:
                                # Truncate long text for better display
                                cell_str = str(cell)
                                if len(cell_str) > 100 and not "id" in str(
                                    column_names
                                ):
                                    row_display.append(f"{cell_str[:97]}...")
                                else:
                                    row_display.append(cell_str)

                        table.add_row(*row_display)

                    console.print(table)

        except Exception as e:
            # Handle non-returning queries (e.g., INSERT, UPDATE)
            affected = cursor.rowcount
            console.print(f"[bold green]Query affected {affected} rows.[/bold green]")
            conn.commit()

    except Exception as e:
        console.print(f"[red]Error executing query: {e}[/red]")


@cli.command()
@click.option(
    "--tag-prefix",
    default="",
    help="Filter tags by prefix (e.g., 'source:', 'entity:')",
)
@click.pass_context
def list_tags(ctx, tag_prefix):
    """List all tags in the database with counts."""

    # Get connection from context
    conn = ctx.obj["conn"]

    try:
        cursor = conn.cursor()

        # Get all tags from documents and text units with counts
        cursor.execute(
            """
            WITH doc_tags AS (
                SELECT UNNEST(tags) as tag FROM documents
            ),
            text_tags AS (
                SELECT UNNEST(tags) as tag FROM text_units
            ),
            all_tags AS (
                SELECT tag FROM doc_tags
                UNION ALL
                SELECT tag FROM text_tags
            )
            SELECT tag, COUNT(*) as count
            FROM all_tags
            WHERE tag LIKE %s || '%%'
            GROUP BY tag
            ORDER BY count DESC, tag
        """,
            (tag_prefix,),
        )

        results = cursor.fetchall()

        if not results:
            console.print(
                f"[yellow]No tags found{' with prefix ' + tag_prefix if tag_prefix else ''}.[/yellow]"
            )
            return

        console.print(f"[bold green]Found {len(results)} unique tags:[/bold green]")

        table = Table(show_header=True, header_style="bold")
        table.add_column("Tag")
        table.add_column("Count")

        for row in results:
            table.add_row(row[0], str(row[1]))

        console.print(table)

    except Exception as e:
        console.print(f"[red]Error listing tags: {e}[/red]")


@cli.command()
@click.argument("nl-query")
@click.option(
    "--max-steps",
    default=5,
    type=int,
    help="Maximum reasoning steps (1 for simple queries)",
)
@click.option("--verbose", is_flag=True, help="Show detailed output")
@click.option(
    "--model",
    default="openai/gpt-4o",
    help="Model to use (e.g., openai/gpt-4o, anthropic/claude-3-opus-20240229)",
)
@click.option("--no-cache", is_flag=True, help="Disable LLM caching")
@click.pass_context
def ask(ctx, nl_query, max_steps, verbose, model, no_cache):
    """
    Ask a natural language question about your documents.

    Example questions:

    # Find recent conversations:
    What are the most recent Slack messages in the engineering channel?

    # Find information about a topic:
    Show me documents discussing machine learning from the last month

    # Find content by a specific author:
    What has John been talking about in the last week?

    # Complex analytical questions:
    Which authors are most active across different channels?

    # Entity-based queries:
    Find documents mentioning GPT-4 and vector databases together
    """
    # Get connection from context
    conn = ctx.obj["conn"]

    init_dspy()
    init_mlflow()

    # Process the query
    process_query(
        conn, nl_query, max_steps, verbose, model_name=model, no_cache=no_cache
    )


@cli.command()
@click.option(
    "--max-steps",
    default=5,
    type=int,
    help="Maximum reasoning steps (1 for simple queries)",
)
@click.option("--verbose", is_flag=True, help="Show detailed output")
@click.option(
    "--model",
    default="openai/gpt-4o",
    help="Model to use (e.g., openai/gpt-4o, anthropic/claude-3-opus-20240229)",
)
@click.option("--no-cache", is_flag=True, help="Disable LLM caching")
@click.option("--show-history", is_flag=True, help="Show query history at startup")
@click.pass_context
def chat(ctx, max_steps, verbose, model, no_cache, show_history):
    """
    Start an interactive chat session with your documents.

    This opens a REPL (Read-Eval-Print Loop) where you can enter natural language
    questions about your document database and get immediate answers.

    Use Ctrl+C or type 'exit', 'quit', or 'q' to exit the chat.
    Type 'history' to see your previous queries.
    Type 'help' to see example queries.
    """
    conn = ctx.obj["conn"]

    # Initialize DSPy
    init_dspy()
    init_mlflow()

    # Print welcome message
    console.print("\n[bold magenta]SimpleMemory Interactive Chat[/bold magenta]")
    console.print(
        "Type your questions or 'exit' to quit. Press Ctrl+C to exit anytime."
    )
    console.print(
        "Type 'help' for example queries or 'history' to see your previous questions.\n"
    )

    # Show count of documents and text units
    try:
        with conn.cursor() as cursor:
            cursor.execute("SELECT COUNT(*) FROM documents")
            doc_count = cursor.fetchone()[0]
            cursor.execute("SELECT COUNT(*) FROM text_units")
            text_count = cursor.fetchone()[0]

            console.print(
                f"[dim]Database contains {doc_count} documents and {text_count} text units[/dim]"
            )
    except Exception as e:
        console.print(f"[yellow]Warning: Could not get database stats: {e}[/yellow]")

    # Define help examples
    help_examples = [
        "What are the most recent Slack messages in the engineering channel?",
        "Show me documents discussing machine learning from the last month",
        "What has John been talking about in the last week?",
        "Which authors are most active across different channels?",
        "Find documents mentioning GPT-4 and vector databases together",
        "Summarize the key topics from the engineering channel in the last week",
    ]

    # Show history if requested
    if show_history:
        show_query_history()

    # Chat history for this session
    session_history = []

    # REPL loop
    try:
        while True:
            try:
                # Get input from user with rich prompt
                query = console.input("\n[bold cyan]You:[/bold cyan] ")

                # Handle special commands
                if query.lower() in ["exit", "quit", "q", "bye"]:
                    console.print("[yellow]Exiting chat mode.[/yellow]")
                    break
                elif query.lower() == "help":
                    console.print("\n[bold]Example questions you can ask:[/bold]")
                    for i, example in enumerate(help_examples, 1):
                        console.print(f"[dim]{i}.[/dim] {example}")
                    continue
                elif query.lower() == "history":
                    show_query_history()
                    continue
                elif not query.strip():
                    continue

                # Process and answer the query
                answer = process_query(
                    conn, query, max_steps, verbose, model_name=model, no_cache=no_cache
                )

                # Add to session history
                session_history.append((query, answer))

            except KeyboardInterrupt:
                # Handle Ctrl+C gracefully
                console.print(
                    "\n[yellow]Chat interrupted. Type 'exit' to quit or continue asking questions.[/yellow]"
                )

            except Exception as e:
                console.print(f"[red]Error: {e}[/red]")
                if verbose:
                    console.print(traceback.format_exc())

    except Exception as e:
        console.print(f"\n[red]Fatal error: {e}[/red]")
        if verbose:
            console.print(traceback.format_exc())

    console.print("\n[magenta]Chat session ended. Goodbye![/magenta]")


def show_query_history(max_entries=10):
    """Show recent query history from the JSONL file"""
    history_file = Path("query_history.jsonl")

    if not history_file.exists():
        console.print("[yellow]No query history found.[/yellow]")
        return

    try:
        # Read the history file
        entries = []
        with open(history_file, "r") as f:
            for line in f:
                try:
                    entry = json.loads(line.strip())
                    entries.append(entry)
                except:
                    pass

        # Show most recent entries first
        entries.reverse()

        if not entries:
            console.print("[yellow]Query history is empty.[/yellow]")
            return

        console.print("\n[bold]Recent queries:[/bold]")
        for i, entry in enumerate(entries[:max_entries], 1):
            timestamp = entry.get("timestamp", "")
            query = entry.get("query", "")
            short_answer = entry.get("answer", "")

            # Truncate long answers
            if len(short_answer) > 100:
                short_answer = short_answer[:97] + "..."

            # Format timestamp
            try:
                dt = datetime.fromisoformat(timestamp)
                formatted_time = dt.strftime("%Y-%m-%d %H:%M")
            except:
                formatted_time = timestamp

            console.print(f"[dim]{i}. {formatted_time}[/dim] [cyan]{query}[/cyan]")

        if len(entries) > max_entries:
            console.print(
                f"[dim]...and {len(entries) - max_entries} more queries[/dim]"
            )

    except Exception as e:
        console.print(f"[yellow]Error reading history: {e}[/yellow]")


@cli.result_callback()
@click.pass_context
def close_connection(ctx, result, **kwargs):
    """Close the database connection when the command finishes."""
    if ctx.obj and "conn" in ctx.obj:
        ctx.obj["conn"].close()
        console.print("[dim]Database connection closed.[/dim]")
    return result


if __name__ == "__main__":
    cli(obj={})
