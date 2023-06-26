import json, sys
import numpy as np
import matplotlib.pyplot as plt

x = list(range(100))

what = input()
with open(sys.argv[1], "r") as f:
    y1 = json.load(f)[what]
    print(sum(y1)/len(y1))

with open(sys.argv[2], "r") as f:
    y2 = json.load(f)[what]
    print(sum(y2)/len(y2))

fig, ax = plt.subplots()

ax.set_xticklabels([])

plt.xlabel("Cycle")
plt.ylabel("QoS")

ax.set_yticks([i/10 for i in range(0, 11)])

ax.plot(x, y1, linewidth=2.0, label="QASRE")
ax.plot(x, y2, linewidth=2.0, label="k8s")
plt.legend(loc='upper center')

ax.set(xlim=(0, 100), xticks=np.arange(0, 100),
       ylim=(0, 7), yticks=np.arange(0, 2))

plt.show()