import {countBy} from 'lodash'
import {passwordHash, authHash, hmacSHA256} from '~/helpers/crypto'

const state = {
  nodeStatus: {syncState: 'DEFAULT'},
  neighbors: []
}

const getters = {
  inBoundCount(state) {
    let res = countBy(state.neighbors, (item) => !item.isOutbound)
    return res.true || 0
  }
}

const mutations = {
  setNodeStatus(state, node) {
    if (!!node){
      state.nodeStatus = node
    }

  },
  setBeneficiaryAddr(state, addr) {
    state.nodeStatus.beneficiaryAddr = addr
  },
  setNeighbors(state, neighbors) {
    if (Array.isArray(neighbors)) {
      state.neighbors = neighbors
    } else {
      state.neighbors = []
    }
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
  async setBeneficiaryAddr({commit, rootState}, {password, beneficiaryAddr}) {
    try {
      this.$axios.setHeader("Authorization", passwordHash(password, hmacSHA256(authHash(password),rootState.token + rootState.unix)))
      let res = await this.$axios.put('/api/node/beneficiary', {beneficiaryAddr: beneficiaryAddr})
      return res.data
    } catch (e) {
      if (e.response.status === 400) {
        e.code = e.response.status
        throw e
      }
      return undefined
    }
  },
  async getNeighbors({commit}) {
    try {
      let res = await this.$axios.get('/api/node/neighbors')
      commit('setNeighbors', res.data)
      return res.data
    } catch (e) {
      throw e
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
