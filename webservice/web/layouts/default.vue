<template>
  <v-app>
    <v-navigation-drawer class="border-right card-border" v-model="drawer" :mini-variant="miniVariant"
                         :clipped="clipped" fixed floating app>
      <v-toolbar flat class="transparent">
        <v-list>
          <v-list-tile>
            <v-list-tile-title class="title">
              My wallet
            </v-list-tile-title>
          </v-list-tile>
        </v-list>
      </v-toolbar>
      <div class="divider"></div>
      <v-container grid-list-lg>
        <v-layout row wrap>
          <v-flex xs12>
            <Wallet></Wallet>
          </v-flex>
          <v-flex xs12>
            <Wallet2></Wallet2>
          </v-flex>
          <v-flex xs12>
            <AddWallet></AddWallet>
          </v-flex>
        </v-layout>
      </v-container>
    </v-navigation-drawer>
    <v-toolbar :clipped-left="clipped" dark app tabs color="indigo" dense>
      <v-toolbar-side-icon @click="drawer = !drawer">
        <v-icon class="fas fa-wallet"></v-icon>
      </v-toolbar-side-icon>
      <div class="divider-vertical mx-3"></div>
      <v-toolbar-title class="ml-2">
        <v-avatar tile size="30">
          <img src="~/static/img/logo.png" alt="avatar">
        </v-avatar>
        NKN WEB
      </v-toolbar-title>
      <v-spacer></v-spacer>
      <NodeRunStatus></NodeRunStatus>
      <div class="divider-vertical mx-3"></div>
      <v-badge color="transparent">
        <span>Node version: v0.8.0-alpha-174-gd090</span>
      </v-badge>
      <template v-slot:extension>
        <v-tabs align-with-title color="transparent">
          <v-tab href="#dashboard" to="/">Dashboard</v-tab>
          <v-tab href="#node_status" to="/node-status">Node status</v-tab>
          <v-tab href="#settings" to="/settings">Settings</v-tab>
          <v-tabs-slider color="pink accent-3"></v-tabs-slider>
        </v-tabs>
      </template>
      <div class="divider-vertical mx-3"></div>
      <v-menu offset-y>
        <template v-slot:activator="{ on }">
          <v-btn icon v-on="on">
            <v-icon>language</v-icon>
          </v-btn>
        </template>
        <v-list>
          <v-list-tile @click="">
            <v-list-tile-title>English</v-list-tile-title>
          </v-list-tile>
          <v-list-tile @click="">
            <v-list-tile-title>简体中文</v-list-tile-title>
          </v-list-tile>
        </v-list>
      </v-menu>
    </v-toolbar>
    <v-content>
      <v-container fluid grid-list-lg>
        <nuxt/>
      </v-container>
      <Footer/>
    </v-content>
  </v-app>
</template>

<script>
  import Footer from '~/components/Footer.vue'
  import Wallet from '~/components/wallet/Wallet.vue'
  import Wallet2 from '~/components/wallet/Wallet2.vue'
  import AddWallet from '~/components/wallet/AddWallet.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import '~/styles/status.scss'

  export default {
    components: {
      Footer,
      Wallet,
      Wallet2,
      AddWallet,
      NodeRunStatus
    },
    data() {
      return {
        clipped: false,
        drawer: true,
        miniVariant: false
      }
    }
  }
</script>
