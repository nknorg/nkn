import {doubleSha256} from '~/helpers/crypto'

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
  async setBeneficiaryAddr({commit}, {password, beneficiaryAddr}) {
    try {
      this.$axios.setHeader("Authorization", doubleSha256(password))
      let res = await this.$axios.put('/api/node/beneficiary',{beneficiaryAddr: beneficiaryAddr})
      return res.data
    }catch(e){
      if (e.response.status === 400) {
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
