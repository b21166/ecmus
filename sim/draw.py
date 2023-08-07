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

# ax.set_xticklabels([])

plt.xlabel("cycle")
plt.ylabel("edge resource usage")

ax.set_yticks([i/10 for i in range(0, 10)])
ax.set_xticks([i for i in range(0, 101, 10)])

box = ax.get_position()
ax.set_position([box.x0, box.y0, box.width * 0.8, box.height])

ax.plot(x, y1, linewidth=2.0, label="QASRE")
ax.plot(x, y2, linewidth=2.0, label="k8s")
ax.legend(loc='center left', bbox_to_anchor=(1, 0.5))

ax.set(xlim=(0, 99),
       ylim=(0, 1))

plt.show()