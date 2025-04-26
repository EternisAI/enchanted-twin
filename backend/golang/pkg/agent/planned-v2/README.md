# Planned Agent V2

A unified implementation of the agent execution system based on the Temporal workflow engine.

## Purpose of the Refactoring

This package is a complete redesign of the planned agent execution system, addressing several limitations in the previous implementations while unifying the scattered functionality into a cohesive, maintainable package. The refactoring streamlines the agent execution flow, provides better integration with modern LLM APIs, and creates a more robust foundation for complex agent workflows.

## Key Improvements

Compared to the previous implementations, the new unified system offers:

1. **OpenAI-Compatible Message Format** - Full compatibility with standardized formats used by modern LLMs, eliminating the need for custom parsing and formatting.

2. **Unified State Model** - A single, comprehensive state structure that combines the best elements from both previous implementations, making state management more predictable.

3. **Simplified Tool Registration** - Streamlined tool definition and registration process with validation and consistent integration with the root agent system.

4. **Enhanced Error Handling** - Comprehensive error propagation and handling throughout the workflow with clear reporting in the state.

5. **Improved Signal & Query Handlers** - Redesigned handler registration with better concurrency patterns and more robust implementations.

6. **Native Tool Call Support** - Direct support for tool calls in the format expected by modern LLMs without additional parsing layers.

7. **Structured History** - Better tracking of execution history with typed entries for improved debugging and UI presentation.

## Directory Structure

```
pkg/agent/planned-v2/
├── activities.go    # LLM and tool execution activities
├── core.go          # Main workflow implementation and ReAct loop
├── handlers.go      # Signal and query handler registration
├── planned_test.go  # Comprehensive test suite
├── README.md        # Package documentation
├── register.go      # Workflow and activity registration helpers
├── state.go         # State definitions and data structures
└── tools.go         # Tool execution logic
```

## Main Features

1. **Flexible Agent Execution**
   - Support for arbitrary plans with configurable max steps
   - Dynamic tool registration and execution
   - Concurrent signal handling for real-time updates

2. **Modern LLM Integration**
   - Support for OpenAI-compatible message format
   - Direct tool calling without intermediate parsing
   - Automatic conversion between internal and API formats

3. **Robust State Management**
   - Comprehensive state tracking for all aspects of execution
   - Queryable execution state for monitoring
   - Detailed history for debugging and audit purposes

4. **Advanced Tool Framework**
   - Standardized tool definition interface
   - Automatic parameter validation
   - Activity-based execution for deterministic behavior
   - Long-running tools (sleep, sleep_until) for timed operations

5. **Extensible Architecture**
   - Easy to add new tools and capabilities
   - Clear separation of concerns between components
   - Well-defined interfaces for future enhancements
   - Support for durable, long-running operations

## Integration with Existing System

The new implementation integrates with the existing system in several ways:

1. **Root Workflow Integration**
   - Registers with the root workflow for centralized management
   - Exposes standard query and signal interfaces
   - Compatible with the existing tool registry

2. **AI Service Compatibility**
   - Uses the standard AI service interface for LLM interactions
   - Supports the same model options as the rest of the system
   - Maintains consistent logging and error reporting

3. **Type System Alignment**
   - Uses shared types from the agent package where appropriate
   - Maintains compatibility with existing tool definitions
   - Preserves required interfaces for interoperability

## Migration Guide

To migrate from the previous implementations to the new unified system:

### From Original Planned Package

1. **Workflow Invocation**:
   ```go
   // Old
   err := client.ExecuteWorkflow(ctx, options, "PlanAgentWorkflow", blueprint, input)
   
   // New
   planInput := plannedv2.PlanInput{
       Plan: planText,
       ToolNames: toolNames,
       Model: "claude-3-sonnet-20240229",
       MaxSteps: 10,
   }
   inputBytes, _ := json.Marshal(planInput)
   err := client.ExecuteWorkflow(ctx, options, plannedv2.WorkflowName, inputBytes)
   ```

2. **State Queries**:
   ```go
   // Old
   var state planned.PlanState
   err := client.QueryWorkflow(ctx, workflowID, runID, "get_state", nil, &state)
   
   // New
   var state plannedv2.PlanState
   err := client.QueryWorkflow(ctx, workflowID, runID, plannedv2.QueryGetState, nil, &state)
   ```

3. **Signal Handling**:
   ```go
   // Old
   err := client.SignalWorkflow(ctx, workflowID, runID, "update_plan", newPlan)
   
   // New
   err := client.SignalWorkflow(ctx, workflowID, runID, plannedv2.SignalUpdatePlan, newPlan)
   ```

### From Root Integration

1. **Tool Registration**:
   ```go
   // Old
   rootAgent.RegisterTool("plan_agent", toolDef)
   
   // New - uses the same interface, but with updated constant names
   rootAgent.RegisterTool(plannedv2.WorkflowName, toolDef)
   ```

2. **Worker Setup**:
   ```go
   // Old
   worker.RegisterWorkflow(planned.PlanAgentWorkflow)
   
   // New - with unified registration
   plannedv2.RegisterPlannedAgentWorkflow(worker, logger)
   ```

Note that the new implementation is designed to be a drop-in replacement for most use cases, with improved capabilities and more consistent behavior. Existing workflows will continue to function, and new workflows should use the new implementation for future development.

## Long-Running Tools

The planned agent includes tools that support long-running operations through Temporal's durability:

### Sleep Tool

Pauses the workflow execution for a specified duration:

```json
{
  "tool": "sleep",
  "params": {
    "duration": 60,
    "reason": "Waiting for external process to complete"
  }
}
```

- `duration`: Number of seconds to sleep (required)
- `reason`: Optional explanation for the sleep

The maximum sleep duration is capped at 24 hours for safety.

### Sleep Until Tool

Pauses the workflow execution until a specific time:

```json
{
  "tool": "sleep_until",
  "params": {
    "timestamp": "2023-12-31T23:59:59Z",
    "reason": "Waiting until year end"
  }
}
```

- `timestamp`: ISO8601 timestamp to sleep until (required, format: `YYYY-MM-DDThh:mm:ssZ`)
- `reason`: Optional explanation for the sleep

The maximum sleep duration is also capped at 24 hours, even if the timestamp is farther in the future.

These tools leverage Temporal's workflow state persistence, allowing agents to pause execution for extended periods without consuming resources, then automatically resume at the appropriate time.