import {passwordHash, seedHash, hmacSHA256} from '~/helpers/crypto'
import {ServiceStatusEnum} from '~/helpers/consts'

const state = {
  currentWalletStatus: {}
}

const getters = {}

const mutations = {
  setCurrentWalletStatus(state, wallet) {
    if (!!wallet) {
      state.currentWalletStatus = wallet
    }
  }
}
const actions = {
  async getCurrentWalletStatus({commit}) {
    try {
      let res = await this.$axios.get('/api/current-wallet/status')
      commit('setCurrentWalletStatus', res.data)
    } catch (e) {
      console.log(e)
    }
  },
  async getCurrentWalletDetails({commit, rootState}, payload) {
    try {
      let seed = localStorage.getItem('seed')
      this.$axios.setHeader("Authorization", passwordHash(payload, hmacSHA256(seed,rootState.token + rootState.unix)))
      let res = await this.$axios.get('/api/current-wallet/details')
      return res.data
    } catch (e) {
      if (e.response.status === 401 || e.response.status === 403) {
        e.code = e.response.status
        throw e
      }
      return undefined
    }
  },
  async createWallet({commit, rootState}, {password, beneficiaryAddr}) {
    try {
      let res = await this.$axios.post('/api/wallet/create', {password: password, beneficiaryAddr:beneficiaryAddr})
      localStorage.setItem('seed', seedHash(password))
      commit('syncServiceStatus', {status: ServiceStatusEnum.SERVICE_STATUS_RUNNING}, {root: true})
      return res.data
    } catch (e) {
      if (e.response.status === 400) {
        e.code = e.response.status
        throw e
      }
    }
  },
  async openWallet({commit, rootState}, payload) {
    try {
      let res = await this.$axios.post('/api/wallet/open', {password: payload})
      localStorage.setItem('seed', seedHash(payload))
      commit('syncServiceStatus', {status: ServiceStatusEnum.SERVICE_STATUS_RUNNING}, {root: true})
      return res.data
    } catch (e) {
      if (e.response.status === 401 || e.response.status === 403) {
        e.code = e.response.status
        throw e
      }
      return undefined
    }
  }
}
export default {
  namespaced: true,
  state: () => state,
  getters,
  actions,
  mutations
}
