import numpy as np
import matplotlib.pyplot as plt
from scipy import sparse

def random_state(N, p):
  """
  Generate column vector with 1 and -1 to represent network state
  N: number of nodes in the network
  p: fraction of nodes that has state 1
  """
  m = int(N * p)
  s = np.concatenate([np.ones(m), np.ones(N-m) * -1]).astype(np.int8)
  np.random.shuffle(s)
  return s

def random_weight(N, k, self_link=True):
  """
  Generate the symmetric sparse connectivity matrix of the network
  N: number of nodes in the network
  k: number of nodes that any node is connected, should be odd for majority vote
  Note: symmetric not enforced due to efficiency
  """
  row = np.arange(N*k, dtype=np.int) / k
  col = np.random.randint(0, N, N*k)
  weight = np.ones(N*k)
  w = sparse.csr_matrix((weight, (row, col)), shape=(N, N), dtype=np.int8)
  if self_link:
    w += sparse.identity(N, dtype=np.int8)
  return w

def next_state(s, w, offset=-0.5):
  """
  Compute the next state give current state vector s and connectivity matrix w
  Offset ensures balanced input becomes 1 or -1
  Set offset to 0 to allow state=0
  """
  return np.sign(w.dot(s) + offset).astype(np.int8)

def agreement_precent(s):
  """
  Compute the percentage of the nodes that has positive state (agree)
  """
  return 1.0 * np.sum(s > 0) / s.shape[0]

def run_simulation(s0, w, num_iters, keep_initial=0, metric_func=np.mean, ylabel='Average State'):
  """
  Run simulation with initial state s0 and weight w for num_iters iterations
  keep_initial controls whether certain state will not change during simulation
  if keep_initial > 0, then all positive state in s0 will not change
  if keep_initial < 0, then all negative state in s0 will not change
  """
  iters = range(num_iters + 1)
  metric = np.zeros(num_iters + 1)
  metric[0] = metric_func(s0)
  s = s0
  for i in iters[1:]:
    s = next_state(s, w)
    if keep_initial > 0:
      s[s0 > 0] = s0[s0 > 0]
    elif keep_initial < 0:
      s[s0 < 0] = s0[s0 < 0]
    metric[i] = metric_func(s)
  plt.plot(iters, metric, '.-', linewidth=2)
  plt.xlabel('Iteration', fontsize=16)
  plt.ylabel(ylabel, fontsize=16)
  plt.tick_params(labelsize=16)
  plt.locator_params(axis='y', nbins=5)

N = 1000000
k = 10
w = random_weight(N, k, self_link=True)

# Convergence for different initial state
N = 1000000
plt.figure(figsize=(12, 5))

k = 10
w = random_weight(N, k, self_link=True)
plt.subplot(1, 2, 1)
plt.title(r'$k=%d$' % k, fontsize=16)
for p in np.arange(0.1, 1.0, 0.1):
  run_simulation(random_state(N, p), w, 10)

k = 100
w = random_weight(N, k, self_link=True)
plt.subplot(1, 2, 2)
plt.title(r'$k=%d$' % k, fontsize=16)
for p in np.arange(0.1, 1.0, 0.1):
  run_simulation(random_state(N, p), w, 10)

plt.tight_layout()
plt.savefig('convergence.svg')

# Tolerance to malicious nodes
N = 1000000
plt.figure(figsize=(12, 5))

k = 10
w = random_weight(N, k, self_link=True)
plt.subplot(1, 2, 1)
plt.title(r'$k=%d$' % k, fontsize=16)
for p in np.arange(0.6, 0.8, 0.02):
  run_simulation(
      random_state(N, p), w, 20, keep_initial=-1,
      metric_func=agreement_precent, ylabel='Fraction of Correct Nodes',
  )

k = 100
w = random_weight(N, k, self_link=True)
plt.subplot(1, 2, 2)
plt.title(r'$k=%d$' % k, fontsize=16)
for p in np.arange(0.6, 0.8, 0.02):
  run_simulation(
      random_state(N, p), w, 20, keep_initial=-1,
      metric_func=agreement_precent, ylabel='Fraction of Correct Nodes',
  )

plt.tight_layout()
plt.savefig('malicious.svg')
