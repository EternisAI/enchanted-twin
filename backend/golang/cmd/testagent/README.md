# Test Agent CLI

This command-line tool helps test the Root Agent workflow. It can create agents, start agent runs, send signals to running agents, and list registered agents and runs.

## Usage

```
testagent [options]
```

### Options

- `-cmd`: Command to execute (create_agent, start_agent, signal_agent, list_agents, list_runs, stop)
- `-agent-id`: Agent ID for the command
- `-blueprint`: Path to blueprint JSON file (for create_agent)
- `-run-id`: Run ID (for signal_agent)
- `-signal`: Signal name (for signal_agent)
- `-payload`: JSON payload (for signal_agent or start_agent)

### Examples

#### Interactive Mode

Run without arguments to start interactive mode:

```
testagent
```

#### Create an Agent

```
testagent -cmd create_agent -agent-id test_agent -blueprint test_blueprint.json
```

#### Start an Agent

```
testagent -cmd start_agent -agent-id test_agent -payload '{"message": "Hello, Agent!"}'
```

#### Send a Signal to an Agent

```
testagent -cmd signal_agent -run-id <run_id> -signal test_signal -payload '{"message": "new data"}'
```

#### List Registered Agents

```
testagent -cmd list_agents
```

#### List Active Runs

```
testagent -cmd list_runs
```

#### Stop the Root Workflow

```
testagent -cmd stop
```

## Test Blueprint

The tool includes a sample blueprint file (`test_blueprint.json`) that can be used for testing.