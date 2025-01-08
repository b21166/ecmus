# KubeDSM (named ecmus before)
For more details you can check out the [paper](https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/KubeDSM.pdf), it is in the process of finishing.
*KubeDSM* is a Kubernetes scheduler module designed to operate on cloud-assisted edge clusters. The following figure displays an overview of the cluster:
<p align="center">
  <img src="https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/ClusterOverview.png" width="80%" height="80%">
</p>
KubeDSM's goal is to minimize user response time by using edge resources as much as possible. 
To do so, KubeDSM is dynamically migrating pods from cloud to edge. Also, it will migrate pods between edge nodes, to reduce resource fragmentation. This will help the upcoming pods to fit better in the edge.
The scheduler is also able to do batch scheduling (i.e. scheduling multiple pods simultaneously).
<p align="center">
  <img src="https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/ComponentDiagram.png" width="50%" height="50%">
</p>


## Paper
For now, you can find the paper [here](https://github.com/AUT-Cloud-Lab/ecmus/blob/main/archive/KubeDSM.pdf) in its early stages.
