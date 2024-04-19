```mermaid
---
title: Diagram the lifecycle states
---
stateDiagram-v2
	direction LR
	
	[*]-->Idle
    Idle-->Running
    Running-->Idle
    Running-->Shutdown
    Shutdown-->[*]
```
