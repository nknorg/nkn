import {startLoopTask} from '~/helpers/task'

const loopInterval = 2000
const syncUnixInterval = 180000
const tickInterval = 1000
export default function ({store}) {
  // loop task
  startLoopTask(function () {
    store.dispatch('node/getNodeStatus')
  }, loopInterval)
  startLoopTask(function () {
    store.dispatch('wallet/getCurrentWalletStatus')
  }, loopInterval)
  startLoopTask(function () {
    store.dispatch('node/getNeighbors')
  }, loopInterval)

  startLoopTask(function () {
    store.dispatch('getToken')
  }, syncUnixInterval)
  startLoopTask(function () {
    store.commit('tick')
  }, tickInterval)
}
