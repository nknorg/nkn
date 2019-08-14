<template>
  <v-dialog v-model="visible" persistent max-width="600px">
    <v-card>
      <v-card-title>
        <span class="headline">{{$t('BENEFICIARY')}}</span>
        <v-spacer></v-spacer>
        <v-btn class="mr-0" icon small @click="cancel">
          <v-icon class="fas fa-times" color="grey darken-2" small></v-icon>
        </v-btn>
      </v-card-title>
      <div class="divider"></div>
      <v-window v-model="step">
        <v-window-item :value="1">
          <v-form ref="form1" @submit.prevent="next">
            <v-card-text>
              <v-text-field autofocus
                            ref="password"
                            v-model="password"
                            :label="$t('PASSWORD') + '*'"
                            :hint="$t('PASSWORD_HINT')"
                            persistent-hint
                            :append-icon="showPassword ? 'visibility' : 'visibility_off'"
                            :type="showPassword ? 'text' : 'password'"
                            :rules="rules.password"
                            :error="passwordError"
                            :error-messages="passwordErrorMessage"
                            @click:append="showPassword = !showPassword"
              ></v-text-field>
            </v-card-text>
          </v-form>
        </v-window-item>
        <v-window-item :value="2">
          <v-form ref="form2" @submit.prevent="next">
            <v-card-text>
              <v-text-field autofocus
                            v-model="beneficiaryAddr"
                            :label="$t('BENEFICIARY')"
                            :hint="$t('settings.BENEFICIARY_HINT')"
                            persistent-hint
                            :error="beneficiaryAddrError"
                            :error-messages="beneficiaryAddrErrorMessage"
              ></v-text-field>
            </v-card-text>
          </v-form>
        </v-window-item>

      </v-window>

      <v-card-actions class="pa-3">
        <v-btn color="blue darken-1" text @click="cancel">{{$t('CANCEL')}}</v-btn>
        <v-spacer></v-spacer>
        <v-btn v-if="step === 3" color="primary" @click="cancel">{{$t('CLOSE')}}</v-btn>
        <v-btn v-else color="primary" @click="next">{{$t('NEXT')}}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>

  import {mapActions} from 'vuex'

  export default {
    name: "BeneficiaryAddr",
    components: {},
    props: {
      value: {
        type: Boolean,
        default: false
      },
      onSuccess:{
        type: Function
      }
    },
    data() {
      return {
        step: 1,
        visible: this.value,
        beneficiaryAddr: '',
        password: '',
        showPassword: false,
        passwordError: false,
        passwordErrorMessage: '',
        beneficiaryAddrError: false,
        beneficiaryAddrErrorMessage: '',
        rules: {
          password: [
            v => !!v || this.$t('PASSWORD_REQUIRED'),
          ],
          // beneficiaryAddr: [
          //   v => !!v || this.$t('settings.BENEFICIARY_REQUIRED'),
          // ]
        }
      }
    },
    watch: {
      value(val) {
        this.visible = val
        if (this.visible === false) {
          this.step = 1
          this.password = ''
          this.passwordError = false
          this.passwordErrorMessage = ''
          this.beneficiaryAddr = ''
          this.beneficiaryAddrError = false
          this.beneficiaryAddrErrorMessage = ''
          this.$refs.form1 && this.$refs.form1.reset()
          this.$refs.form2 && this.$refs.form2.reset()
        }
      },
      password(val) {
        this.passwordError = false
        this.passwordErrorMessage = ''
      },
    },
    methods: {
      ...mapActions('node', ['setBeneficiaryAddr']),
      ...mapActions(['verification']),
      cancel() {
        this.visible = false
        this.$emit('input', this.visible)
      },

      async next() {
        if (this.step === 1) {
          if (this.$refs.form1.validate()) {
            try {
              await this.verification(this.password)
              this.passwordError = false
              this.passwordErrorMessage = ''
              this.step++
            } catch (e) {
              if (e.code === 401 || e.code === 403) {
                this.passwordError = true
                this.passwordErrorMessage = this.$t('PASSWORD_ERROR')
              }
            }
          }
        } else if (this.step === 2) {
          if (this.$refs.form2.validate()) {
            try {
              let res = await this.setBeneficiaryAddr({password: this.password, beneficiaryAddr: this.beneficiaryAddr})
              this.beneficiaryAddrError = false
              this.beneficiaryAddrErrorMessage = ''
              if (typeof this.onSuccess === 'function'){
                this.onSuccess(res)
              }
              this.cancel()
            } catch (e) {
              if (e.code === 400) {
                this.beneficiaryAddrError = true
                this.beneficiaryAddrErrorMessage = this.$t('settings.BENEFICIARY_ERROR')
              }
            }
          }
        }

      }
    }
  }
</script>

<style scoped>

</style>
