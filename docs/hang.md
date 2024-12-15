# Hang

For design of GPU architecture and programming model, GPU programs are often executed asynchronously, and then probably **Hang**. When Hang problem occurs, CPU/GPU is busy polling, and there is no special output log, so it is difficult to detect. For example, it is difficult to distinguish whether a process is Hang or sleep inf. Therefore we analyzed and thought about manifestation of Hang problem and set up a series of indicators to detect occurrence of Hang problem. The currently selected indicators are:

- high GPU power
- high graph clock frequency
- high sm clock frequency
- high sm utilization
- low memory throughput
- low pviol (power violation)
- low PCI TX/RX bandwidth

High utilization indicators can avoid misjudgment of cases such as sleep, and low utilization indicators can avoid misjudgment of cases such as normal training, thereby maximizing the efficiency and accuracy of Hang problem diagnosis.
