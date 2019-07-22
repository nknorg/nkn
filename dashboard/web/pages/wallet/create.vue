<template>
    <v-container fluid grid-list-lg>
        <v-form ref="form">
            <v-card class="card">
                <v-card-title class="headline">
                    {{$t('WALLET_CREATE')}}
                </v-card-title>
                <div class="divider"></div>
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
                    <v-text-field
                            ref="password_confirm"
                            v-model="passwordConfirm"
                            :label="$t('PASSWORD_CONFIRM') + '*'"
                            :hint="$t('PASSWORD_HINT')"
                            persistent-hint
                            :append-icon="showPasswordConfirm ? 'visibility' : 'visibility_off'"
                            :type="showPasswordConfirm ? 'text' : 'password'"
                            :rules="rules.passwordConfirm"
                            :error="passwordConfirmError"
                            :error-messages="passwordConfirmErrorMessage"
                            @click:append="showPasswordConfirm = !showPasswordConfirm"
                    ></v-text-field>

                    <v-text-field
                                  v-model="beneficiaryAddr"
                                  :label="$t('BENEFICIARY') + (beneficiaryAddrRequired && '*')"
                                  :hint="$t('settings.BENEFICIARY_HINT')"
                                  persistent-hint
                                  :rules="rules.beneficiaryAddr"
                                  :error="beneficiaryAddrError"
                                  :error-messages="beneficiaryAddrErrorMessage"
                    ></v-text-field>
                </v-card-text>

                <v-card-actions class="pa-3">
                    <v-spacer></v-spacer>
                    <v-btn color="primary" small @click="submit">{{$t('CREATE')}}</v-btn>
                </v-card-actions>
            </v-card>
        </v-form>
    </v-container>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex'
  import '~/styles/sign.scss'

  export default {
    layout: 'sign',
    name: "create",
    computed: {
      ...mapGetters({
        beneficiaryAddrRequired: 'beneficiaryAddrRequired'
      }),
      ...mapState({
        stateBeneficiaryAddr: state => state.beneficiaryAddr
      })
    },
    data: function () {
      return {
        password: '',
        passwordConfirm: '',
        showPassword: false,
        showPasswordConfirm: false,
        passwordError: false,
        passwordConfirmError: false,
        passwordErrorMessage: '',
        passwordConfirmErrorMessage: '',
        beneficiaryAddr: '',
        beneficiaryAddrError: false,
        beneficiaryAddrErrorMessage: '',

        rules: {
          password: [
            v => !!v || this.$t('PASSWORD_REQUIRED'),
          ],
          passwordConfirm: [
            v => !!v || this.$t('PASSWORD_CONFIRM_REQUIRED'),
            v => this.password === v || this.$t('PASSWORD_CONFIRM_ERROR'),
          ],
          beneficiaryAddr: [
            v => !this.beneficiaryAddrRequired || !!v || this.$t('settings.BENEFICIARY_REQUIRED'),
          ]
        }
      }
    },
    created(){
      this.$nextTick(() =>  {
        this.beneficiaryAddr = this.stateBeneficiaryAddr
      })
    },
    methods: {
      ...mapActions('wallet', ['createWallet']),
      async submit() {
        sessionStorage.setItem('seed','')
        if (this.$refs.form.validate()) {
          try {
            await this.createWallet({password: this.password, beneficiaryAddr: this.beneficiaryAddr})
            this.$router.push(this.localePath('loading'))
          } catch (e) {
            this.beneficiaryAddrError = true
            this.beneficiaryAddrErrorMessage = this.$t('settings.BENEFICIARY_ERROR')
          }
        }
      }
    }
  }
</script>

<style scoped>

</style>
