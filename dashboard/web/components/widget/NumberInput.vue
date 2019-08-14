<template>
    <v-layout>

        <v-flex md3>
            <v-text-field :label="label" :value="value" :step="step" @input="onInput" type="text"
                          :suffix="suffix"
                          persistent-hint
                          :rules="rules.number"
                          :error="numberError"
                          :error-messages="numberErrorMessage"
            >
            </v-text-field>
        </v-flex>
        <v-flex md9 d-flex align-end>
            <v-slider
                    v-model="val"
                    @change="onValueChange"
                    :color="color"
                    track-color="grey"
                    always-dirty
                    :min="min"
                    :max="max"
                    :step="step"
            >
                <template v-slot:prepend>
                    <v-icon :color="color" @click="decrement">
                        mdi-minus
                    </v-icon>
                </template>
                <template v-slot:append>
                    <v-icon :color="color" @click="increment">
                        mdi-plus
                    </v-icon>
                </template>
            </v-slider>
        </v-flex>

    </v-layout>
</template>

<script>
  import {sub, add} from '~/helpers/math'

  export default {
    name: "NumberInput",
    props: {
      value: {
        default: 0
      },
      min: {
        type: Number,
        default: 0
      },
      max: {
        type: Number,
        default: 1
      },
      step: {
        type: Number,
        default: 0.01
      },
      label: {
        type: String
      },
      suffix: {
        type: String
      }
    },
    computed: {
      color() {
        if (this.val / this.max < 0.1) return 'grey'
        if (this.val / this.max < 0.4) return 'green'
        if (this.val / this.max < 0.6) return 'indigo'
        if (this.val / this.max < 0.8) return 'orange'
        return 'red'
      }
    },
    data() {
      return {
        val: this.value,
        numberError: false,
        numberErrorMessage: '',
        rules: {
          number: [
            v => {

              if (v === '')
                return this.$t('NUMBER_FIELD_ERROR')
              let num = Number(v)
              if (isNaN(num)) {
                return this.$t('NUMBER_FIELD_ERROR')
              } else if (num < 0) {
                return this.$t('POSITIVE_NUMBER_FIELD_ERROR')
              } else {
                return true
              }
            },
          ],
        }
      }
    },
    watch: {
      value(v) {
        this.onInput(v)
      }
    },
    methods: {
      onInput(v) {
        this.$emit('input', v)
        let num = parseFloat(v)
        if (num < this.min) {
          this.val = this.min
        } else if (num > this.max)
          this.val = this.max
        else
          this.val = num

      },
      onValueChange() {
        this.$emit('input', this.val)
      },
      decrement() {
        if (this.val <= this.min) {
          return
        }
        this.val = sub(this.val, this.step).toNumber()
        this.onValueChange()
      },
      increment() {
        if (this.val >= this.max) {
          return
        }
        this.val = add(this.val, this.step).toNumber()
        this.onValueChange()
      },
    }
  }
</script>

<style scoped>

</style>
