<template>
    <v-layout>

        <v-flex md4>
            <v-text-field :label="label" :value="value" :step="step" @input="onInput" type="text"
                          :suffix="suffix"
                          persistent-hint
                          :rules="rules.number"
                          :error="numberError"
                          :error-messages="numberErrorMessage"
            >
                <template v-slot:append>
                    <slot name="append-icon"></slot>
                </template>
                <template v-slot:append-outer>
                    <slot name="append-outer"></slot>
                </template>
            </v-text-field>
        </v-flex>
        <v-flex md8 d-flex align-end>
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
  import {Decimal} from 'decimal.js'
  import {sub, add, NKN_DP_LENGTH} from '~/helpers/math'

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
      numberMin: {
        type: Number,
        default: 0
      },
      numberMax: {
        type: Number,
        default: Number.MAX_SAFE_INTEGER
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
              } else if (num < this.numberMin) {
                return this.$t('NUMBER_MIN_ERROR', {min: this.numberMin})
              } else if (num > this.numberMax) {
                return this.$t('NUMBER_MAX_ERROR', {max: this.numberMax})
              } else if (new Decimal(num).dp() > NKN_DP_LENGTH) {
                return this.$t('NUMBER_DP_ERROR', {dp: NKN_DP_LENGTH})
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
