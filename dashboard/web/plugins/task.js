import {startLoopTask} from '~/helpers/task'

export default function ({store}) {
  // loop task
  startLoopTask(function () {
    store.dispatch('node/getNodeStatus')
  }, 1000)
  startLoopTask(function () {
    store.dispatch('wallet/getCurrentWalletStatus')
  }, 1000)
  startLoopTask(function () {
    store.dispatch('node/getNeighbors')
  }, 1000)
}
