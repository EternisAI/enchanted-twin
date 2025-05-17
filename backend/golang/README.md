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

### Vector database (Weaviate)

The Go backend starts an **embedded Weaviate server** at runtime (see `bootstrapWeaviateServer` in `cmd/server/main.go`).

• It serves on the port defined by the `WEAVIATE_PORT` environment variable (defaults to `51414`).

