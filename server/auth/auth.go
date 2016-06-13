package auth

import (
	"bytes"
	"encoding/json"
	"github.com/ventu-io/slf"
	"io"
	"os"
)

const pwdCurr string = "github.com/go-stomp/stomp/server/auth"

type AuthDB struct {
	configAuthDB string
	log          slf.StructuredLogger
	db           map[string]string // Map with login in key and password in value for authentification field
}

type AuthParams struct {
	Login    string `json:Login`
	Passcode string `json:Passcode`
}

func NewAuth(fileWithLogins string) *AuthDB {
	a := AuthDB{configAuthDB: fileWithLogins, log: slf.WithContext(pwdCurr)}
	a.initAuthDB()

	return &a
}

func (a *AuthDB) Authenticate(login, passcode string) bool {
	a.log.Debugf("login: %s, pwd: %s ", login, passcode)
	if pwd, ok := a.db[login]; ok {
		if pwd == passcode {
			return true
		}
	}
	return false
}

// Get Login/Passcode dataBase from configure file
// Read JSON data and parsing it to AuthParams struct
func (a *AuthDB) initAuthDB() {

	buf := bytes.NewBuffer(nil)

	fp, err := os.Open(a.configAuthDB)
	if err != nil {
		a.log.Errorf("Could not read data from configureAuthFile: %s ", err.Error())
	}
	defer fp.Close()

	_, err = io.Copy(buf, fp)
	if err != nil {
		a.log.Errorf("Could not process data from configureAuthFile: %s ", err.Error())
	}

	authDataJSON := buf.Bytes()
	//log.Println("authDataJSON: ", string(authDataJSON))

	authData := []AuthParams{}

	err = json.Unmarshal(authDataJSON, &authData)
	if err != nil {
		a.log.Errorf("Couldn't get auth params from configureAuthFile: %s", err.Error())
	}

	dataMap := make(map[string]string)
	for _, userAuth := range authData {
		if len(dataMap) != 0 {
			if _, userExist := dataMap[userAuth.Login]; userExist {
				a.log.Warn("User already exists in database; ignored")
				continue
			}
		}
		dataMap[userAuth.Login] = userAuth.Passcode
	}
	a.db = dataMap
}