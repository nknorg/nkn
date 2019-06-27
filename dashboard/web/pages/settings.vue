<template>
  <v-card>
    <v-card-title class="headline">Settings</v-card-title>
    <div class="divider"></div>
    <v-card-text>
      <v-layout wrap>
        <v-flex xs12>
          <v-subheader class="pa-0">Restart nknd</v-subheader>
          <NodeRunStatus :node-status="this.nodeStatus"></NodeRunStatus>
          <v-btn color="primary" :loading="restarting"
                 :disabled="restarting" @click="restarting = !restarting">RESTART
          </v-btn>
        </v-flex>
        <v-flex xs12>
          <v-subheader class="pa-0">Choose a wallet for run nknd?</v-subheader>
        </v-flex>
        <v-flex xs12 md6>
          <v-combobox
            v-model="wallets"
            :hint="!isEditWallet ? 'Click the icon to edit' : 'Click the icon to save'"
            :readonly="!isEditWallet"
            :items="myWallets"
            label="Choose"
            item-text="address"
            item-value="address"
            persistent-hint
          >

            <template v-slot:item="data">
              <template v-if="typeof data.item !== 'object'">
                <v-list-tile-content v-text="data.item"></v-list-tile-content>
              </template>
              <template v-else>
                <v-chip color="primary" dark>{{data.item.name}}</v-chip>
                <v-list-tile-content>
                  <v-list-tile-sub-title>{{data.item.address}}</v-list-tile-sub-title>
                </v-list-tile-content>
              </template>
            </template>
            <template v-slot:append-outer>
              <v-btn fab small :color="isEditWallet ? 'success' : 'info'">
                <v-icon
                  :key="`icon-${isEditWallet}`"
                  @click="isEditWallet = !isEditWallet"
                  v-text="isEditWallet ? 'check' : 'edit'"
                ></v-icon>
              </v-btn>
            </template>
          </v-combobox>
        </v-flex>
        <v-flex xs12>
          <v-subheader class="pa-0">Input a beneficiary address for auto transfer.</v-subheader>
        </v-flex>
        <v-flex xs12 md6>
          <v-text-field
            label="Beneficiary"
            hint="New amount will auto transfer to this address."
            persistent-hint
          ><template v-slot:append-outer>
            <v-btn fab small color="success">
              <v-icon>check</v-icon>
            </v-btn>
          </template></v-text-field>
        </v-flex>
      </v-layout>
    </v-card-text>
    <div class="divider"></div>
    <v-card-actions class="pa-3">

      <v-btn color="primary">SUBMIT</v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
import {mapState} from 'vuex'
  export default {
    name: "settings",
    components: {
      NodeRunStatus
    },
    computed: mapState({
      nodeStatus: state => state.node.nodeStatus,
    }),
    data: () => ({
      isEditWallet: false,
      restarting: false,
      wallets: null,



      myWallets: [
        {name: 'NKN Wallet', address: 'sssdfsfsdfdsfsd', color:'#ff00ff'},
        {name: 'Main Wallet', address: 'sssdfsfsdfds1fsd', color:'#f0903f'},
        {name: 'Wallet 2', address: 'sssdfsfsdfd2sfsd', color:'#b2b2b2'},
        {name: 'Wallet 3', address: 'sssdfsfsdf3dsfsd', color:'#3e3e3e'},

      ],
      title: 'The summer breeze'
    }),
    methods: {
      remove(item) {
        const index = this.wallets.indexOf(item.name)
        if (index >= 0) this.wallets.splice(index, 1)
      }
    }
  }
</script>

<style scoped>

</style>
