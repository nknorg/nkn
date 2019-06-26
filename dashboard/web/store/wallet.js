const state = {
  currentWalletStatus: {}
}

const getters = {

}

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

    // console.log(res.data)
    // commit('setNodeStatus', {version: '11.1.1'})
  }
}
export default {
  namespaced: true,
  state,
  getters,
  actions,
  mutations
}
