import sys
import json
import typing
import numpy as np

data_path = sys.argv[1]
output_path = sys.argv[2]

with open(data_path, "r") as f:
    data = json.load(f)

apis = data["apis"]
cycle_length = data["cycle_length"]
number_of_cycles = data["number_of_cycles"]
threshold = data["threshold"]

for api in apis:
    mean = api.pop("mean")
    sigma = api.pop("sigma")
    replicas = list(np.random.normal(mean, sigma, number_of_cycles))
    for i in range(len(replicas)):
        replicas[i] = max(replicas[i], 1)
        replicas[i] = round(replicas[i])
    
    intervals = []
    def get_interval(replica, cnt):
        interval = {}

        period = replica * threshold
        length = cnt * cycle_length
        interval["api_call_period"] = period
        interval["cycle_length"] = length

        return interval

    cnt = 1
    last_replica = replicas[0]
    for i in range(1, len(replicas)):
        if last_replica == replicas[i]:
            cnt += 1
            continue
        
        intervals.append(get_interval(last_replica, cnt))
        
        last_replica = replicas[i]
        cnt = 1

    intervals.append(get_interval(last_replica, cnt))
    api["intervals"] = intervals

with open(output_path, "w") as f:
    json.dump({"apis": apis}, f, indent=4)
