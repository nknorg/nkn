const state = {
  nodeStatus: {syncState: 'DEFAULT'}
}

const getters = {}

const mutations = {
  setNodeStatus(state, node) {
    state.nodeStatus = node
  },
  setBeneficiaryAddr(state, addr) {
    state.nodeStatus.beneficiaryAddr = addr
  }
}
const actions = {
  async getNodeStatus({commit}) {
    try {
      let res = await this.$axios.get('/api/node/status')
      commit('setNodeStatus', res.data)
    } catch (e) {
      console.log(e)
    }
  },
  async setBeneficiaryAddr({commit}, payload) {
    try {
      this.$axios.setHeader("Authorization", payload)
      let res = await this.$axios.put('/api/current-wallet/beneficiary')
      return res.data
    }catch(e){
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
  state,
  getters,
  actions,
  mutations
}
