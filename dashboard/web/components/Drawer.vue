<template>
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
</template>

<script>
  export default {
    name: "Drawer",
    data: () =>({
      clipped: false,
      drawer: true,
      miniVariant: false
    })
  }
</script>

<style scoped>

</style>
