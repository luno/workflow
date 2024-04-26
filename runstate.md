```mermaid
---
title: Diagram the run states of a workflow
---
stateDiagram-v2
	direction LR
    
	[*]-->Initiated

    Initiated-->Running
    
    Running-->Completed
    Running-->Paused
    Running-->Cancelled

    Paused-->Running
    Paused-->Cancelled
    
    Completed-->DataDeleted
    Cancelled-->DataDeleted

    DataDeleted-->DataDeleted
    DataDeleted-->[*]
```
