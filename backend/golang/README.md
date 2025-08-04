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

### Integration tests memory 

Command exmaple 
```
TEST_SOURCE=misc TEST_INPUT_PATH=testdata/misc make test-integration
```

note: add your data in backend/golang/pkg/dataprocessing/integration/testdata