<template>
    <v-container fluid grid-list-lg>
        <v-form ref="form" @submit.prevent="submit">
            <v-card class="card">
                <v-card-title class="headline">
                    {{$t('WALLET_OPEN')}}
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

                </v-card-text>

                <v-card-actions class="pa-3">
                    <v-spacer></v-spacer>
                    <v-btn color="primary" small @click="submit">{{$t('OPEN')}}</v-btn>
                </v-card-actions>
            </v-card>
        </v-form>
    </v-container>
</template>

<script>
  import {mapActions} from 'vuex'
  import '~/styles/sign.scss'

  export default {
    layout: 'sign',
    name: "walletOpen",
    data: function () {
      return {
        password: '',
        showPassword: false,
        passwordError: false,
        passwordErrorMessage: '',
        rules: {
          password: [
            v => !!v || this.$t('PASSWORD_REQUIRED'),
          ]
        }
      }
    },
    methods: {
      ...mapActions('wallet', ['openWallet']),
      async submit() {
        sessionStorage.setItem('seed','')
        if (this.$refs.form.validate()) {
          try {
            await this.openWallet(this.password)
            this.$router.push(this.localePath('loading'))
          } catch (e) {
            this.passwordError = true
            this.passwordErrorMessage = this.$t('PASSWORD_ERROR')
          }
        }
      }
    }
  }
</script>

<style scoped>

</style>
