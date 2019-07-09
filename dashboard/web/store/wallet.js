import {passwordHash} from '~/helpers/crypto'

const state = {
  currentWalletStatus: {}
}

const getters = {}

const mutations = {
  setCurrentWalletStatus(state, wallet) {
    state.currentWalletStatus = wallet
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
  async getCurrentWalletPrivateKey({commit,rootState}, payload) {
    try {
      this.$axios.setHeader("Authorization", passwordHash(payload, rootState.token + rootState.unix))
      let res = await this.$axios.get('/api/current-wallet/details')
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
