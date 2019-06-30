import Vue from 'vue'
import Vuex from 'vuex'
import common from './common'
import node from './node'
import wallet from './wallet'

Vue.use(Vuex)

const store = {
  modules: {
    node,
    wallet
  },
  ...common
}

export default store

