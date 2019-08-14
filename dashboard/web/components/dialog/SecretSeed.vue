<template>
  <v-dialog v-model="visible" persistent max-width="600px">
    <v-card>
      <v-card-title>
        <span class="headline">{{$t('SECRET_SEED')}}</span>
        <v-spacer></v-spacer>
        <v-btn class="mr-0" icon small @click="cancel">
          <v-icon class="fas fa-times" color="grey darken-2" small></v-icon>
        </v-btn>
      </v-card-title>
      <div class="divider"></div>
      <v-window v-model="step">
        <v-window-item :value="1">
          <v-form ref="form" @submit.prevent="next">
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
          <v-card-text>
            <ClipboardText v-model="secretSeed" :label="$t('SECRET_SEED')"></ClipboardText>
          </v-card-text>
        </v-window-item>

      </v-window>

      <v-card-actions class="pa-3">
        <v-btn color="blue darken-1" text @click="cancel">{{$t('CANCEL')}}</v-btn>
        <v-spacer></v-spacer>
        <v-btn v-if="step === 1" color="primary" @click="next">{{$t('NEXT')}}</v-btn>
        <v-btn v-if="step === 2" color="primary" @click="cancel">{{$t('CLOSE')}}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
  import ClipboardText from '~/components/widget/ClipboardText.vue'
  import {mapActions} from 'vuex'

  export default {
    name: "SecretSeed",
    components: {
      ClipboardText
    },
    props: {
      value: {
        type: Boolean,
        default: false
      }
    },
    data: function () {
      return {
        step: 1,
        visible: this.value,
        password: '',
        secretSeed: '',
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
    watch: {
      value(val) {
        this.visible = val
        if (this.visible === false) {
          this.step = 1
          this.$refs.form.reset()
        }
      },
      password(val) {
        this.passwordError = false
        this.passwordErrorMessage = ''
      },
    },
    methods: {
      ...mapActions('wallet', ['getCurrentWalletDetails']),
      cancel() {
        this.visible = false
        this.$emit('input', this.visible)
      },
      // onCopySuccess(e){
      //
      // },
      async next() {
        if (this.$refs.form.validate()) {
          try {
            let res = await this.getCurrentWalletDetails(this.password)
            this.secretSeed = res.secretSeed
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
        return false
      }
    }
  }
</script>

<style scoped>

</style>
