import {doubleSha256} from '~/helpers/crypto'

const state = {}

const getters = {}

const mutations = {}
const actions = {
  async verification({commit}, payload) {
    try {
      this.$axios.setHeader("Authorization", doubleSha256(payload))
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
  state :() => state,
  getters,
  actions,
  mutations
}
