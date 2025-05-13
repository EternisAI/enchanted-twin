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
