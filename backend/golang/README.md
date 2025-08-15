## Go backend

### Temporal

Schedule agent task

```sh
temporal workflow start \
  --task-queue default \
  --workflow-id scheduled-task-$(uuidgen) \
  --type TaskScheduleWorkflow \
  --input '{"task": "Send me a joke"}'
```

### Vector database (PostgreSQL + pgvector)

The Go backend uses **embedded PostgreSQL with pgvector** for vector storage and similarity search.

• It serves on the port defined by the `POSTGRES_PORT` environment variable (defaults to `5432`).

### Logging

JSON structured logging is enabled by default. Configure logging with environment variables:

#### Global Configuration
```bash
export LOG_FORMAT=json          # json, text, or logfmt
export LOG_LEVEL=info          # debug, info, warn, error
```

#### Component-Specific Log Levels
```bash
# Set log levels per component
export LOG_LEVEL_HOLON_MANAGER=debug
export LOG_LEVEL_AI_ANONYMIZER=warn
export LOG_LEVEL_WHATSAPP_SERVICE=error
```

**Note**: Component IDs use dot notation (e.g., `holon.manager`, `ai.anonymizer`) but environment variables use uppercase with underscores (e.g., `LOG_LEVEL_HOLON_MANAGER`, `LOG_LEVEL_AI_ANONYMIZER`). The system automatically maps between these formats.

#### Available Components
To see all registered components, use:
```bash
./bin/enchanted-twin --list-components
```

#### Filtering Logs with jq
```bash
# Filter by specific component
jq 'select(.component == "holon.manager")' logs.json

# Filter by log level  
jq 'select(.level == "error")' logs.json

# Filter by component prefix (e.g., all AI components)
jq 'select(.component | startswith("ai."))' logs.json

# Multiple filters
jq 'select(.component == "ai.service" and .level == "error")' logs.json

# Find all components with prefix
jq -r 'select(.component | startswith("holon.")) | .component' logs.json | sort | uniq
```

### Integration tests memory 

Command exmaple 
```
TEST_SOURCE=misc TEST_INPUT_PATH=testdata/misc make test-integration
```

note: add your data in backend/golang/pkg/dataprocessing/integration/testdata