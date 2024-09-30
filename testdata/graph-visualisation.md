```mermaid
---
title: Diagram of example Workflow
---
stateDiagram-v2
	direction LR
	
	9: Start
	10: Middle
	11: End
	
    state if_state <<choice>>
    9 --> if_state
    if_state --> 10
    if_state --> 11 
	
	10-->11
```