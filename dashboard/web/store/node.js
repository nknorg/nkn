const state = {
  nodeStatus: {syncState:'DEFAULT'}
}

const getters = {

}

const mutations = {
  setNodeStatus(state, node) {
    state.nodeStatus = node
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
