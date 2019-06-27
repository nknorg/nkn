<template>
  <v-text-field :label="label" :value="value" box readonly :hide-details="hideDetails">
    <template v-slot:append>
      <v-btn icon small v-clipboard:copy="value">
        <v-icon class="far fa-copy" small></v-icon>
      </v-btn>
    </template>
  </v-text-field>
</template>

<script>
  export default {
    name: "clipboardText",
    props: {
      value: {
        type: String
      },
      label:{
        type: String
      },
      hideDetails:{
        type:Boolean
      }
    },
    data() {
      return {

      }
    },
    methods: {
      onCopySuccess(e) {

      }
    }
  }
</script>
