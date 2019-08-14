<template>
    <v-card>
        <v-card-title class="headline">{{$t('settings.TITLE')}}</v-card-title>
        <div class="divider"></div>
        <v-card-text>
            <v-layout wrap>
                <v-flex xs12>
                    <v-subheader class="pa-0">{{$t('settings.TITLE')}}</v-subheader>
                    <NodeRunStatus :node-status="this.nodeStatus"></NodeRunStatus>
                    <!--<v-btn color="primary" :loading="restarting"
                           :disabled="restarting" @click="restarting = !restarting">{{$t('RESTART')}}
                    </v-btn>-->
                </v-flex>
                <v-flex xs12 md6>
                    <v-subheader class="pa-0">{{$t('settings.BENEFICIARY_TITLE')}}</v-subheader>
                    <v-text-field :label="$t('BENEFICIARY')" :value="nodeStatus.beneficiaryAddr" filled readonly
                                  persistent-hint
                                  :hint="$t('settings.BENEFICIARY_HINT')">
                        <template v-slot:append>
                            <v-btn icon small v-clipboard:copy="nodeStatus.beneficiaryAddr">
                                <v-icon color="primary" class="far fa-copy" small></v-icon>
                            </v-btn>
                            <v-btn icon small @click="beneficiaryAddrDialog = true">
                                <v-icon color="primary" class="far fa-edit" small></v-icon>
                            </v-btn>
                        </template>
                    </v-text-field>
                </v-flex>

            </v-layout>

            <v-form ref="form">
                <v-subheader class="pa-0">{{$t('settings.CONFIG_TITLE')}}</v-subheader>
                <v-layout wrap>
                    <v-flex xs12 md6>

                        <NumberInput :label="$t('config.Register_ID_Txn_Fee')"
                                     v-model="registerIDTxnFee"
                                     :min="0"
                                     :max="10"
                                     :step="0.01"
                        ></NumberInput>
                    </v-flex>
                </v-layout>
                <v-layout wrap>
                    <v-flex xs12 md6>
                        <NumberInput :label="$t('config.Low_Fee_Txn_Size_Per_Block')"
                                     v-model="lowFeeTxnSizePerBlock"
                                     suffix="kb"
                                     :min="0"
                                     :max="102400"
                                     :step="1"
                        ></NumberInput>
                    </v-flex>
                </v-layout>
                <v-layout wrap>
                    <v-flex xs12 md6>
                        <NumberInput :label="$t('config.Num_Low_Fee_Txn_Per_Block')"
                                     v-model="numLowFeeTxnPerBlock"
                                     :min="0"
                                     :max="100"
                                     :step="1"
                        ></NumberInput>
                    </v-flex>
                </v-layout>
                <v-layout wrap>
                    <v-flex xs12 md6>
                        <NumberInput :label="$t('config.Min_Txn_Fee')"
                                     v-model="minTxnFee"
                                     :min="0"
                                     :max="10"
                                     :step="0.01"
                        ></NumberInput>
                    </v-flex>
                </v-layout>
            </v-form>
        </v-card-text>
        <div class="divider"></div>
        <v-card-actions class="pa-3">
            <v-btn color="primary" @click="submit">{{$t('SUBMIT')}}</v-btn>
            <v-btn color="primary" text @click="reset">{{$t('RESET')}}</v-btn>
        </v-card-actions>
        <BeneficiaryAddrDialog v-model="beneficiaryAddrDialog" :on-success="onDialogSuccess"></BeneficiaryAddrDialog>
        <VerificationPassword v-model="submitDialog" :title="$t('SUBMIT')"
                              :on-success="onSubmitSuccess"></VerificationPassword>
        <Result v-model="submitResultDialog" :type="submitResult" :timeout="3000"></Result>
    </v-card>
</template>

<script>
  import BeneficiaryAddrDialog from '~/components/dialog/BeneficiaryAddr.vue'
  import VerificationPassword from '~/components/dialog/VerificationPassword.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import MaterialNotification from '~/components/material/Notification.vue'
  import NumberInput from '~/components/widget/NumberInput.vue'
  import Result from '~/components/dialog/Result.vue'
  import {formatFee, parseFee} from '~/helpers/tools'
  import {mapState, mapActions} from 'vuex'

  export default {
    name: "settings",
    components: {
      NodeRunStatus,
      BeneficiaryAddrDialog,
      VerificationPassword,
      MaterialNotification,
      Result,
      NumberInput
    },
    computed: {
      ...mapState({
        nodeStatus: state => state.node.nodeStatus,
      })
    },
    data() {
      return {
        loaded: false,
        beneficiaryAddrDialog: false,
        submitDialog: false,
        submitResult: 'error',
        submitResultDialog: false,
        restarting: false,
        wallets: null,
        beneficiaryAddr: '',
        registerIDTxnFee: 0,
        numLowFeeTxnPerBlock: 0,
        lowFeeTxnSizePerBlock: 0,
        minTxnFee: 0
      }
    },
    created() {
      this.reset()
    },
    methods: {
      ...mapActions('node', ['setNodeConfig']),
      onDialogSuccess(data) {
        this.beneficiaryAddr = data.beneficiaryAddr
      },
      reset() {
        this.registerIDTxnFee = this.nodeStatus.registerIDTxnFee
        this.numLowFeeTxnPerBlock = this.nodeStatus.numLowFeeTxnPerBlock
        this.lowFeeTxnSizePerBlock = this.nodeStatus.lowFeeTxnSizePerBlock
        this.minTxnFee = formatFee(this.nodeStatus.minTxnFee)
      },
      submit() {
        if (this.$refs.form.validate()) {
          this.submitDialog = true
        }
      },
      async onSubmitSuccess(data) {
        try {
          let res = await this.setNodeConfig({
            password: data, config: {
              registerIDTxnFee: Number(this.registerIDTxnFee),
              numLowFeeTxnPerBlock: Number(this.numLowFeeTxnPerBlock),
              lowFeeTxnSizePerBlock: Number(this.lowFeeTxnSizePerBlock),
              minTxnFee: parseFee(this.minTxnFee)
            }
          })
          this.submitResult = 'success'
          this.submitResultDialog = true
        } catch (e) {
          console.log(e)
          this.submitResult = 'error'
          this.submitResultDialog = true
        }
      }
    },
    watch: {
      nodeStatus(v) {
        if (!this.loaded) {
          this.loaded = true
          this.reset()
        }
      }
    }
  }
</script>

<style scoped>

</style>
