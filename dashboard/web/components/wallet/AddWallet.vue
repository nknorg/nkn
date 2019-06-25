<template>
  <v-card color="grey" class="white--text wallet">
    <div class="badge tag grey darken-2">
      <div class="text">Add Wallet</div>
    </div>
    <v-toolbar class="toolbar" color="grey" card dark flat dense>
      <v-spacer></v-spacer>
      <v-btn icon small disabled>
        <v-icon class="fas fa-cog" small></v-icon>
      </v-btn>
      <v-btn icon small disabled>
        <v-icon class="fas fa-times" small></v-icon>
      </v-btn>
      <div class="line"></div>
    </v-toolbar>
    <v-card-title>
      <v-text-field dark
                    label="Open wallet"

                    v-model="fileName"
                    box
                    readonly>
        <template v-slot:append>
          <v-btn icon small @click="pickFile">
            <v-icon small>attach_file</v-icon>
          </v-btn>

        </template>
      </v-text-field>
      <input type="file" style="display: none" ref="walletFile" @change="onFilePicked">
      <v-text-field dark
                    label="Password"
                    box
                    type="password">
        <template v-slot:append>
          <v-btn icon small>
            <v-icon class="far fa-eye" small></v-icon>
          </v-btn>

        </template>
      </v-text-field>
    </v-card-title>
    <div class="divider"></div>
    <v-card-actions class="pa-3">
      <v-btn flat dark small>CREATE</v-btn>
      <v-spacer></v-spacer>
      <v-btn color="primary" dark small>OPEN</v-btn>
    </v-card-actions>
  </v-card>
</template>

<script>
  import '~/styles/wallet.scss'

  export default {
    name: "AddWallet",
    data: () => ({
      fileName: '',
      file: ''
    }),
    methods: {
      pickFile() {
        this.$refs.walletFile.click()
      },
      onFilePicked(e) {
        const files = e.target.files
        this.fileName = files[0].name
        if (this.fileName !== undefined) {
          if (files[0].name.lastIndexOf('.') <= 0) {
            return
          }
          const fr = new FileReader ()
          fr.readAsDataURL(files[0])
          fr.addEventListener('load', () => {
            // this.fileData = fr.result
            this.file = files[0] // this is an image file that can be sent to server...
          })
        } else {
          console.log('no picked file')
        }
      }
    }
  }
</script>

