```mermaid
---
title: Workflow diagram of example
---
stateDiagram-v2
	direction LR
	
	9: Start
	10: Middle
	11: End
	
    state 9_branching <<choice>>
    9 --> 9_branching
    9_branching --> 10
    9_branching --> 11 
	
	10-->11
```