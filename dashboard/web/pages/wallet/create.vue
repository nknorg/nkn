<template>
  <v-container fluid grid-list-lg>
    <v-form ref="form" @submit.prevent="submit">
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
          </v-card-text>

          <v-card-actions class="pa-3">
            <v-spacer></v-spacer>
            <v-btn color="primary" small submit>{{$t('CREATE')}}</v-btn>
          </v-card-actions>
        </v-card>
    </v-form>
  </v-container>
</template>

<script>
  import '~/styles/sign.scss'
  export default {
    layout: 'sign',
    name: "cteateWallet",
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
        rules: {
          password: [
            v => !!v || this.$t('PASSWORD_REQUIRED'),
          ],
          passwordConfirm: [
            v => !!v || this.$t('PASSWORD_CONFIRM_REQUIRED'),
            v => this.password === v || this.$t('PASSWORD_CONFIRM_ERROR'),
          ]
        }
      }
    },
    methods: {
      submit() {
        if (this.$refs.form.validate()) {

        }
      }
    }
  }
</script>

<style scoped>

</style>
