import {passwordHash} from '~/helpers/crypto'
import {ServiceStatusEnum} from '~/helpers/consts'


const state = {
  token: undefined,
  unix: Date.now(),
  serviceStatus: ServiceStatusEnum.SERVICE_STATUS_DEFAULT
}

const getters = {}

const mutations = {
  syncServiceStatus(state, data) {
    state.serviceStatus = data.status
  },
  syncToken(state, data) {
    state.token = data.token
    state.unix = data.unix
    console.log(`sync service unix: ${state.unix}, token: ${state.token}`)
  },
  syncUnix(state, unix) {
    state.unix = unix
    console.log('sync service unix: ' + state.unix)
  },
  tick(state) {
    state.unix++
  }
}
const actions = {
  async getServiceStatus({commit}) {
    try {
      let res = await this.$axios.get('/api/sync/status')
      commit('syncServiceStatus', res.data)
    } catch (e) {
      if (e.response.status !== 200) {
        e.code = e.response.status
        throw e
      }
    }
  },
  async getToken({commit}) {
    try {
      let res = await this.$axios.get('/api/sync/token')
      commit('syncToken', res.data)
    } catch (e) {
      if (e.response.status !== 200) {
        e.code = e.response.status
        throw e
      }
    }
  },
  async getUnix({commit}) {
    try {
      let res = await this.$axios.get('/api/sync/unix')
      commit('syncUnix', parseInt(res.data.unix))
    } catch (e) {
      if (e.response.status === 400) {
        e.code = e.response.status
        throw e
      }
    }
  },
  async verification({commit, rootState}, payload) {
    try {
      this.$axios.setHeader("Authorization", passwordHash(payload, rootState.token + rootState.unix))
      let res = await this.$axios.head('/api/verification')
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
  state: () => state,
  getters,
  actions,
  mutations
}
