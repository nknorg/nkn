import {startLoopTask} from '~/helpers/task'
import {ServiceStatusEnum} from '~/helpers/consts'

const loopInterval = 2000
const syncUnixInterval = 5000
const tickInterval = 1000
export default function ({store}) {
  startLoopTask(function () {
    store.dispatch('getServiceStatus')
  }, loopInterval)
  startLoopTask(function () {
    store.dispatch('getToken')
  }, syncUnixInterval)
  startLoopTask(function () {
    store.commit('tick')
  }, tickInterval)

  startLoopTask(function () {
    if ((store.state.serviceStatus & ServiceStatusEnum.SERVICE_STATUS_CREATE_ID) > 0 || (store.state.serviceStatus & ServiceStatusEnum.SERVICE_STATUS_RUNNING) > 0) {
      store.dispatch('wallet/getCurrentWalletStatus')
      store.dispatch('node/getNodeStatus')
      store.dispatch('node/getNeighbors')
    }
  }, loopInterval)
}

