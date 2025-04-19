#!/usr/bin/env python3
"""
Slack Data Processor for GraphRAG

This script processes raw Slack export data into a format suitable for GraphRAG.
It extracts message content, author information, and channel details,
ensuring metadata is properly formatted as JSON.
"""

import json
import os
import sys
import logging
from pathlib import Path
from datetime import datetime
from typing import Dict, Any, Optional, List, Union

import click
import pandas as pd
from dateutil.parser import parse as parse_date


# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("slack-processor")


class SlackProcessor:
    """Class to process Slack data into GraphRAG format"""

    def __init__(self, verbose: bool = False):
        """
        Initialize the Slack processor

        Args:
            verbose: Whether to enable verbose logging
        """
        self.verbose = verbose
        if verbose:
            logger.setLevel(logging.DEBUG)

    def process_file(self, input_path: Path, output_path: Path) -> None:
        """
        Process Slack data from CSV file

        Args:
            input_path: Path to input CSV file
            output_path: Path to output processed CSV file
        """
        logger.info(f"Reading data from {input_path}")
        try:
            df = pd.read_json(input_path, lines=True)
        except Exception as e:
            logger.error(f"Failed to read input file: {e}")
            raise
        print(df)

        logger.debug(f"Input columns: {df.columns.tolist()}")
        logger.debug(f"Read {len(df)} records from input file")

        # Check for already processed file
        if self._is_already_processed(df):
            df = self._cleanup_processed_file(df)

        # Process data
        df = self._process_raw_data(df)

        # Save output
        self._save_output(df, output_path)
        logger.info(f"Processing complete. Output saved to {output_path}")

        # Log stats
        self._log_stats(df)

    def _is_already_processed(self, df: pd.DataFrame) -> bool:
        """Check if the file has already been processed"""
        return any(col.endswith(".1") for col in df.columns) or (
            "title" in df.columns and "text" in df.columns
        )

    def _cleanup_processed_file(self, df: pd.DataFrame) -> pd.DataFrame:
        """Clean up an already processed file"""
        logger.info(
            "Input appears to be already processed. Cleaning up duplicated columns..."
        )

        # Remove duplicate columns
        if any(col.endswith(".1") for col in df.columns):
            base_columns = [
                col
                for col in df.columns
                if not col.endswith(".1") and not col.endswith(".2")
            ]
            df = df[base_columns]

        return df

    def _process_raw_data(self, df: pd.DataFrame) -> pd.DataFrame:
        """Process raw Slack data"""
        logger.info("Processing data...")

        # Ensure consistent date column
        if "timestamp" in df.columns and "creation_date" not in df.columns:
            df = df.rename(columns={"timestamp": "creation_date"})

        # Extract message data
        if "data" in df.columns:
            logger.info("Transforming message data...")

            # Transform data
            transformed_data = []
            for _, row in df.iterrows():
                transformed = self._transform_message(row)
                if transformed:
                    # Create new row with original data and transformed data
                    new_row = row.to_dict()
                    new_row.update(transformed)
                    transformed_data.append(new_row)

            # Create new dataframe with transformed data
            if transformed_data:
                df = pd.DataFrame(transformed_data)
            else:
                logger.warning("No valid messages found in input data")
                return pd.DataFrame()

        # Standardize metadata
        if "metadata" in df.columns:
            logger.info("Standardizing metadata...")
            df["metadata"] = df.apply(self._standardize_metadata, axis=1)

        return df

    def _transform_message(self, row: pd.Series) -> Optional[Dict[str, Any]]:
        """
        Accept both dict‑ and str‑typed `data` values and return a normalized record.
        Rows whose data can’t be parsed (or that carry no text) are skipped.
        """
        try:
            # 1. Parse the `data` field -----------------------------------------
            raw = row["data"]

            if isinstance(raw, dict):
                data = raw
            elif isinstance(raw, str):
                try:
                    data = json.loads(raw)
                except json.JSONDecodeError:
                    logger.debug(f"Unparseable data string: {raw!r}")
                    return None
            else:
                logger.debug(f"Unexpected data type: {type(raw)}")
                return None

            # 2. Extract the bits we need ---------------------------------------
            is_my_message = bool(data.get("myMessage"))
            username = (data.get("username") or "UNKNOWN").strip()
            channel_name = data.get("channelName", "").strip()
            content = data.get("text", "").strip()

            # Ignore rows that carry no human‑visible text
            if not content:
                return None

            # Timestamp: prefer explicit columns, fall back to “now”
            ts = (
                row.get("timestamp")
                or row.get("creation_date")
                or datetime.now().isoformat()
            )

            # 3. Build title + metadata -----------------------------------------
            title = (
                f"Slack message from me in #{channel_name}"
                if is_my_message
                else f"Slack message from {username} in #{channel_name}"
            )

            metadata = {
                "source": "slack",
                "author": username,
                "channel": channel_name,
                "created_at": ts,
                "is_my_message": is_my_message,
            }

            return {"title": title, "text": content, "metadata": metadata}

        except Exception as e:
            logger.debug(f"Error transforming row: {e}")
            return None

    def _standardize_metadata(self, row: pd.Series) -> str:
        """Standardize metadata to valid JSON format"""
        metadata = row.get("metadata")

        # Skip if no metadata
        if metadata is None:
            return None

        # If metadata is already a dictionary
        if isinstance(metadata, dict):
            return json.dumps(self._ensure_serializable(metadata))

        # If metadata is a string
        if isinstance(metadata, str):
            try:
                # Try parsing as JSON first
                json.loads(metadata)
                return metadata
            except json.JSONDecodeError:
                # If it looks like a Python dict string
                if metadata.startswith("{") and "'" in metadata:
                    try:
                        import ast

                        parsed = ast.literal_eval(metadata)
                        if isinstance(parsed, dict):
                            return json.dumps(self._ensure_serializable(parsed))
                    except:
                        logger.debug(
                            f"Failed to parse metadata as Python dict: {metadata}"
                        )

        # Fallback: try to convert to JSON anyway
        try:
            return json.dumps(self._ensure_serializable(metadata))
        except:
            logger.warning(f"Could not convert metadata to JSON: {metadata}")
            return json.dumps({})

    def _ensure_serializable(self, obj: Any) -> Any:
        """Ensure object is JSON serializable"""
        if isinstance(obj, dict):
            return {k: self._ensure_serializable(v) for k, v in obj.items()}
        elif isinstance(obj, list):
            return [self._ensure_serializable(item) for item in obj]
        elif isinstance(obj, (int, float, str, bool, type(None))):
            return obj
        elif isinstance(obj, datetime):
            return obj.isoformat()
        else:
            return str(obj)

    def _save_output(self, df: pd.DataFrame, output_path: Path) -> None:
        """Save processed data to output file"""
        # Create output directory if it doesn't exist
        output_dir = output_path.parent
        if not output_dir.exists():
            logger.info(f"Creating output directory: {output_dir}")
            output_dir.mkdir(parents=True, exist_ok=True)

        # Save to CSV
        df.to_csv(output_path, index=False)
        logger.info(f"Saved {len(df)} records to {output_path}")

    def _log_stats(self, df: pd.DataFrame) -> None:
        """Log statistics about the processed data"""
        logger.info(f"Processed {len(df)} messages")

        if "metadata" in df.columns:
            # Get sample metadata
            sample = None
            for meta in df["metadata"]:
                if meta:
                    sample = meta
                    break

            if sample:
                logger.info(f"Sample metadata: {sample}")

                # Validate JSON
                try:
                    json.loads(sample)
                    logger.info("✓ Metadata is valid JSON")
                except json.JSONDecodeError as e:
                    logger.error(f"✗ Invalid JSON in metadata: {e}")

        # Check for author/channel stats if available
        try:
            metadata_dicts = df["metadata"].apply(lambda x: json.loads(x) if x else {})
            authors = metadata_dicts.apply(
                lambda x: x.get("author", "UNKNOWN") if x else "UNKNOWN"
            )
            channels = metadata_dicts.apply(
                lambda x: x.get("channel", "UNKNOWN") if x else "UNKNOWN"
            )

            logger.info(f"Found {authors.nunique()} unique authors")
            logger.info(f"Found {channels.nunique()} unique channels")

            if self.verbose:
                logger.info("\nTop 5 authors:")
                logger.info(authors.value_counts().head(5).to_string())
                logger.info("\nTop 5 channels:")
                logger.info(channels.value_counts().head(5).to_string())
        except:
            pass


@click.command()
@click.option(
    "--input",
    "-i",
    type=click.Path(exists=True, readable=True, file_okay=True, dir_okay=True),
    default="./input_data/slack.csv",
    help="Path to input Slack CSV file",
)
@click.option(
    "--output",
    "-o",
    type=click.Path(file_okay=True, dir_okay=False),
    default="./output_data/slack.csv",
    help="Path to output processed CSV file",
)
@click.option(
    "--verbose", "-v", is_flag=True, default=False, help="Enable verbose output"
)
def main(input: str, output: str, verbose: bool) -> None:
    """Process Slack data from CSV file and transform it to GraphRAG format."""
    try:
        input_path = Path(input).resolve()
        output_path = Path(output).resolve()

        print(f"Input path: {input_path}")
        print(f"Output path: {output_path}")

        input_files = []
        if input_path.is_dir():
            input_files = list(input_path.glob("*.jsonl"))
        else:
            input_files.append(input_path)

        print(f"Input files: {input_files}")

        processor = SlackProcessor(verbose=verbose)
        for input_file in input_files:
            processor.process_file(input_file, output_path)

        sys.exit(0)
    except Exception as e:
        logger.error(f"Error processing Slack data: {e}")
        if verbose:
            import traceback

            logger.error(traceback.format_exc())
        sys.exit(1)


if __name__ == "__main__":
    main()
