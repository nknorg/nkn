import Vue from 'vue'
import Vuex from 'vuex'
import node from './node'
import wallet from './wallet'

Vue.use(Vuex)

const store = () => new Vuex.Store({
  modules: {
    node,
    wallet
  }
})

export default store

