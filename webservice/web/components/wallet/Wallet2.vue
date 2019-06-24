<template>
  <v-card color="cyan darken-2" class="white--text wallet">

    <div class="badge tag success">
      <div class="text">NKN Wallet</div>
    </div>
    <v-toolbar class="toolbar" color="cyan darken-2" card dark flat dense>
      <v-spacer></v-spacer>
      <v-btn icon small>
        <v-icon class="fas fa-cog" small></v-icon>
      </v-btn>
      <v-btn icon small>
        <v-icon class="fas fa-times" small></v-icon>
      </v-btn>
      <div class="line"></div>
    </v-toolbar>
    <v-card-title>
      <v-text-field dark
                    label="Address"
                    placeholder="Address"
                    value="NSwZKAehPzs4DTP7Hugf3C58V4uX4NE5Fa"
                    readonly>
        <template v-slot:append>
          <v-btn icon small>
            <v-icon class="far fa-copy" small></v-icon>
          </v-btn>
        </template>
      </v-text-field>
      <v-text-field dark
                    label="Balance"
                    placeholder="0"
                    value="1230"
                    readonly>
        <template v-slot:prepend-inner>
          <div style="margin-top: 6px">
            <img width="20" src="data:image/png;base64,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">
          </div>
        </template>
        <template v-slot:append>
          <v-btn icon small>
            <v-icon class="far fa-eye" small></v-icon>
          </v-btn>
        </template>
      </v-text-field>
    </v-card-title>

    <div class="divider"></div>
    <v-card-actions class="pa-3">
      <v-btn flat dark small>DETAILS</v-btn>
      <v-spacer></v-spacer>
      <v-btn color="primary"  dark small>TRANSFER</v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
  import '~/styles/wallet.scss'
  export default {
    name: "Wallet2",
    data: ()=>({
      loading: true
    })
  }
</script>
