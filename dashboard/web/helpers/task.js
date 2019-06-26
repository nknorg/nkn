export function startLoopTask(func, timeout) {
  setInterval(func, timeout)
  func()
}
