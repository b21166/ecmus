# KubeDSM (named ecmus before)
For more details you can check out the [paper](https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/KubeDSM.pdf), it's wrapping up :)
## Cluster
*KubeDSM* is a Kubernetes scheduler module designed to operate on cloud-assisted edge clusters. The following figure displays an overview of the cluster:
<p align="center">
  <img src="https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/ClusterOverview.png" width="80%" height="80%">
</p>

## Goal
KubeDSM's goal is to minimize user response time by using edge resources as much as possible. 
To do so, KubeDSM is dynamically migrating pods from cloud to edge. Also, it will migrate pods between edge nodes, to reduce resource fragmentation. This will help the upcoming pods to fit better in the edge.
The scheduler is also able to do batch scheduling (i.e. scheduling multiple pods simultaneously). You can see the scheduler's component diagram in the figure below:
<p align="center">
  <img src="https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/ComponentDiagram.png" width="80%" height="80%">
</p>

## Algorithm
The underlying scheduling algorithm takes advantage of the edge cluster's small resource size to make almost optimal decisions to maximize its goal function (Quality Of Service, QoS).
The algorithm's data flow diagram:
<p align="center">
  <img src="https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/AlgorithmDFD.png" width="80%" height="80%">
</p>
The nature of the edge environment is error-prone and the scheduler is developed to be resilient to this. The scheduler is able to *heal* its state whenever it gets into a malfunctioned state, and it expects errors and interruptions to happen in each step.

## Experiment
The scheduler's performance has been experimented with various generated multi-pattern workloads for hours in a heterogeneous edge cluster connected to a cloud node. The results show that KubeDSM achieved a **50%** reduction in user response time, a **20%** increase in resource utilization, and fulfilled **100%** of theoretically satisfiable QoS constraints.

## Helper projects
The following is the list of the projects developed alongside KubeDSM for this research:
- [Genny](https://github.com/AUT-Cloud-Lab/genny): Genny generates scenarios based on the given config. It is used to generate normal distributed and wavy scenarios.
- [DrStress](https://github.com/AUT-Cloud-Lab/DrStress): DrStress implements the given scenario to generate workload for the scheduler.
- [MineDraft](https://github.com/AUT-Cloud-Lab/MineDraft): MineDraft extracts draft diagrams and tables from the metrics aggregated during the scenario execution, these drafts further are used to analyze and create diagrams for the paper.
- [Sencillo](https://github.com/AUT-Cloud-Lab/sencillo): As Kubernetes default scheduler does not perform well in heterogeneous edge clusters, several scheduling algorithms are implemented in Sencillo to compete with KubeDSM in the experiments.

## Paper
For now, you can find the paper [here](https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/KubeDSM.pdf).
