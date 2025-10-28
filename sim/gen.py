import json
import numpy as np
import random

with open("data.json") as f:
    data = json.load(f)

deployments = {k: np.array(v["resources"]) * v["share"] for k, v in data["deployments"].items()}
edge_resources = np.array(data["edge_resources"])

last_frame = {name: 0 for name in deployments.keys()}
scenario = []
print(deployments)

for _ in range(100):
    candid = set(deployments.keys())
    current_resources = np.zeros(edge_resources.shape)
    
    frame = {name: 0 for name in candid}
    
    while len(candid):
        choice = random.choice(list(candid))
        if any(current_resources + deployments[choice] > edge_resources):
            candid.remove(choice)
        
        frame[choice] += 1
        current_resources += deployments[choice]
    
    deletions = []
    addition = []

    for name in deployments.keys():
        if frame[name] > last_frame[name]:
            addition += [name] * (frame[name] - last_frame[name])
        elif frame[name] < last_frame[name]:
            deletions += [name] * (last_frame[name] - frame[name])
    
    last_frame = frame

    scenario.append({
        "delete_pods": deletions,
        "new_pods": addition
    })

with open("scenario.json", "w") as f:
    json.dump(scenario, f)
