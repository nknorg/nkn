# NKN Golang Style Guide  
v0.1 - Last updated Apr 16, 2018

## Contents  
* [1. gofmt and goimports](#format)
* [2. folders and files](#filename)
* [3. package](#package)
* [4. import](#import)
* [5. comment](#comment)
* [6. function](#function)
* [7. variable](#variable)
* [8. errors](#error)
* [9. others](#others)


## Detail  

<h4 id="format"> 1. gofmt and goimports </h4>

**[rule 1-1] Use gofmt to fix the majority of mechanical style issues.**  
**[rule 1-2] Use goimports to import package automatically.**  

Plug this two tools into your IDE. Whenever you save any .go file, they will automatically fix any unused or missing imports, and format your code for you.  

<h4 id="filename"> 2. folders and files </h4> 

**[rule 2-1] Filenames should be all lowercase.**  
**[rule 2-2] Foldernames should be all lowercase.**  
**[rule 2-3] Test file should be like `xxx_test.go`.**  
**[rule 2-4] Must not use Chinese Pinyin.**  

``` go  
// file  
accountstate.go  
  
// folder  
nkn/core/transaction/payload  
  
// test file  
transaciton_test.go  
```  

<h4 id="package">3. package </h4> 

**[rule 3-1] Package name should be same to the folder name.**  

<h4 id="import">4. import </h4> 

**[rule 4-1] Imports are organized in groups: standard packages, inner packages and third party packages.**  
**[rule 4-2] Groups should be split by blank lines.**  
**[rule 4-3] Import path should begin with `$GOPATH`.**  
**[rule 4-4] must not use alias.**  
 
``` go  
import (  
	"os"  
	"sort"  
  
	"github.com/nknorg/nkn/cli/info"  
	"github.com/nknorg/nkn/cli/wallet"  
  
	"github.com/urfave/cli"  
)  
```  
 
<h4 id="comment">5. comment </h4> 
 
**[rule 5-1] Comments should begin with the name of the thing being described and end in a period.**  
**[rule 5-2] Perfer line comments to block comments.**  
**[rule 5-3] Must not use Chinese Pinyin.**  
 
``` go  
// Encode writes the JSON encoding of req to w.  
func Encode(w io.Writer, req *Request) { ...  
```  
 
<h4 id="function">6. function </h4> 
 
**[rule 6-1] It is better to always use camel-case.**  
**[rule 6-2] The first character should be uppercase if this function is accessible outside a package.**  
**[rule 6-3] Functions should be split by a blank line.**  

``` go
func (tx *Transaction) GetMergedAssetIDValueFromReference() (TransactionResult, error) {
	reference, err := tx.GetReference()
	if err != nil {
		return nil, err
	}
	var result = make(map[Uint256]Fixed64)
	for _, v := range reference {
		amout, ok := result[v.AssetID]
		if ok {
			result[v.AssetID] = amout + v.Value
		} else {
			result[v.AssetID] = v.Value
		}
	}
	return result, nil
}
```
 
<h4 id="variable">7. variable </h4> 

**[rule 7-1] It is better to always use camel-case.**  
**[rule 7-2] The first character should be uppercase if this variable is accessible outside a package.**  
**[rule 7-3] declaring an empty slice, prefer `var t []string`**  

``` go
type Transaction struct {
	TxType         TransactionType
	PayloadVersion byte
	Payload        Payload
	Attributes     []*TxAttribute
	UTXOInputs     []*UTXOTxInput
	Outputs        []*TxOutput
	Programs       []*program.Program
	hash           *Uint256
}
```

<h4 id="error">8. errors</h4> 

**[rule 8-1] The error return code must be handled.**  
**[rule 8-2] Don't use panic for normal error handling.**  
**[rule 8-3] It's better to use error and multiple return values.**  

``` go
err := tx.DeserializeUnsigned(r)
if err != nil {
	log.Error("Deserialize DeserializeUnsigned:", err)
	return NewDetailErr(err, ErrNoCode, "transaction Deserialize error")
}
```

<h4 id="others">9. others</h4> 

**[rule 9-1] Keep code to 80 columns.**  
**[rule 9-2] Use 4-space tabs**  
**[rule 9-3] Must not use magic number.**  

