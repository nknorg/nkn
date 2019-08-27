<template>
    <v-layout flex row wrap>
        <v-flex>
            <v-card min-width="800" class="node-status">
                <v-card-title class="headline">{{$t('node_status.TITLE')}}</v-card-title>
                <div class="divider"></div>
                <v-card-text>
                    <v-layout wrap>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('node_status.NODE_STATUS')}}: </span>
                                </v-flex>
                                <v-flex>
                                    <NodeRunStatus :node-status="this.nodeStatus"></NodeRunStatus>
                                </v-flex>
                            </v-layout>

                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('node_status.NODE_VERSION')}}: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.version}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('node_status.ID')}}: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.id}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('node_status.RELAY_MESSAGE_COUNT')}}: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.relayMessageCount}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('node_status.HEIGHT')}}: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.height}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <v-badge color="transparent" class="breathe mr-4">
                                        <template v-slot:badge>

                                        </template>

                                    </v-badge>
                                    <span>{{$t('node_status.BENEFICIARY_ADDR')}}
                                        <v-tooltip top>
                                          <template v-slot:activator="{ on }">
                                            <v-icon color="grey darken-3" dark small v-on="on">mdi-help-circle</v-icon>
                                          </template>
                                          <span>{{$t('tooltip.BENEFICIARY_TOOLTIP')}}</span>
                                        </v-tooltip>:
                                    </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.beneficiaryAddr}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('config.Register_ID_Txn_Fee')}}
                                        <v-tooltip top>
                                        <template v-slot:activator="{ on }">
                                            <v-icon color="grey darken-3" dark small v-on="on">mdi-help-circle</v-icon>
                                        </template>
                                        <span>{{$t('tooltip.Register_ID_Txn_Fee_TOOLTIP')}}</span>
                                    </v-tooltip>:
                                    </span>
                                </v-flex>
                                <v-flex d-flex algin-center>
                                    <Balance :balance="formatFee(nodeStatus.registerIDTxnFee)"></Balance>
                                </v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('config.Low_Fee_Txn_Size_Per_Block')}}
                                        <v-tooltip top>
                                          <template v-slot:activator="{ on }">
                                            <v-icon color="grey darken-3" dark small v-on="on">mdi-help-circle</v-icon>
                                          </template>
                                          <span>{{$t('tooltip.Low_Fee_Txn_Size_Per_Block_TOOLTIP')}}</span>
                                        </v-tooltip>: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.lowFeeTxnSizePerBlock}} kb</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout wrap>
                                <v-flex shrink class="label">
                                    <span>{{$t('config.Num_Low_Fee_Txn_Per_Block')}}
                                        <v-tooltip top>
                                          <template v-slot:activator="{ on }">
                                            <v-icon color="grey darken-3" dark small v-on="on">mdi-help-circle</v-icon>
                                          </template>
                                          <span>{{$t('tooltip.Num_Low_Fee_Txn_Per_Block_TOOLTIP')}}</span>
                                        </v-tooltip>: </span>
                                </v-flex>
                                <v-flex>{{nodeStatus.numLowFeeTxnPerBlock}}</v-flex>
                            </v-layout>
                        </v-flex>
                        <v-flex xs12>
                            <v-layout class="text" align-center>
                                <v-flex shrink class="label">
                                    <span>{{$t('config.Min_Txn_Fee')}}
                                        <v-tooltip top>
                                          <template v-slot:activator="{ on }">
                                            <v-icon color="grey darken-3" dark small v-on="on">mdi-help-circle</v-icon>
                                          </template>
                                          <span>{{$t('tooltip.Min_Txn_Fee_TOOLTIP')}}</span>
                                        </v-tooltip>: </span>
                                </v-flex>
                                <v-flex d-flex algin-center>
                                    <Balance :balance="formatFee(nodeStatus.minTxnFee)"></Balance>
                                </v-flex>
                            </v-layout>
                        </v-flex>
                    </v-layout>
                </v-card-text>
            </v-card>
        </v-flex>
        <v-flex>
            <v-card min-width="800">
                <v-card-title class="headline">{{$t('current_wallet_status.TITLE')}}
                    <v-spacer></v-spacer>
                    <v-btn icon small>
                        <v-icon color="primary" class="fa fa-download" small
                                @click="verificationPasswordDialog = true"></v-icon>
                    </v-btn>
                </v-card-title>
                <div class="divider"></div>
                <v-card-text>
                    <v-layout wrap>
                        <v-flex xs12 d-flex align-items="center">
                            <v-layout class="text" align-center>
                                <v-flex shrink>{{$t('BALANCE')}}:</v-flex>
                                <Balance :balance="currentWalletStatus.balance || 0"></Balance>
                            </v-layout>
                        </v-flex>
                    </v-layout>
                    <v-layout wrap>
                        <v-flex xs12 md6 xs6>
                            <ClipboardText v-model="currentWalletStatus.address" hideDetails
                                           :label="$t('WALLET_ADDRESS')"></ClipboardText>
                        </v-flex>
                    </v-layout>
                    <v-layout wrap>
                        <v-flex xs12 md6 xs6>
                            <ClipboardText v-model="currentWalletStatus.publicKey" hideDetails
                                           :label="$t('PUBLIC_KEY')"></ClipboardText>
                        </v-flex>
                    </v-layout>
                    <v-layout>
                        <v-flex xs12>
                <span>
                  {{$t('SECRET_SEED')}}:
                  <v-btn icon small>
                    <v-icon class="far fa-eye" small color="primary" @click="secretSeedDialog = true"></v-icon>
                  </v-btn>
                </span>
                        </v-flex>
                    </v-layout>
                </v-card-text>
            </v-card>
        </v-flex>
        <SecretSeedDialog v-model="secretSeedDialog"></SecretSeedDialog>
        <VerificationPasswordDialog v-model="verificationPasswordDialog" :title="$t('WALLET_DOWNLOAD')"
                                    :on-success="onVerificationPasswordSuccess"></VerificationPasswordDialog>
    </v-layout>
</template>

<script>
  import VerificationPassword from '~/components/dialog/VerificationPassword.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import SecretSeed from '~/components/dialog/SecretSeed.vue'
  import ClipboardText from '~/components/widget/ClipboardText.vue'
  import Balance from '~/components/Balance.vue'

  import {mapState, mapActions} from 'vuex'
  import {formatFee} from '~/helpers/tools'
  import '~/styles/node.scss'

  export default {
    name: "nodeStatus",
    components: {
      SecretSeedDialog: SecretSeed,
      VerificationPasswordDialog: VerificationPassword,
      NodeRunStatus,
      ClipboardText,
      Balance
    },
    data: () => ({
      secretSeedDialog: false,
      verificationPasswordDialog: false
    }),
    computed: mapState({
      nodeStatus: state => state.node.nodeStatus,
      currentWalletStatus: state => state.wallet.currentWalletStatus
    }),
    created() {

    },
    methods: {
      formatFee,
      ...mapActions('wallet', ['downloadWallet']),
      remove(item) {
        const index = this.wallets.indexOf(item.name)
        if (nodeStatus >= 0) this.wallets.splice(nodeStatus, 1)
      },
      async onVerificationPasswordSuccess(data) {
        await this.downloadWallet(data)
      }
    }
  }
</script>

<style scoped>

</style>
