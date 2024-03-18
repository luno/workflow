```mermaid
---
title: Diagram of example Workflow
---
stateDiagram-v2
	direction LR
	
	[*]-->Start
	
	Start-->Middle
	Start-->End
	Middle-->End
	
	End-->[*]
```