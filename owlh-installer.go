package main

import (
	"os"
	"bytes"
	"io/ioutil"
	"encoding/json"
	"database/sql"
	"os/exec"
	"strings"	
	"time"	
	"bufio"	
	"regexp"	
	"errors"
	"github.com/astaxie/beego/logs"
	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
    Versionfile		string `json:"versionfile"`
    Masterbinpath 	string `json:"masterbinpath"`
    Masterconfpath 	string `json:"masterconfpath"`
    Mastertarfile 	string `json:"mastertarfile"`
    Nodebinpath 	string `json:"nodebinpath"`
    Nodeconfpath 	string `json:"nodeconfpath"`
    Nodetarfile 	string `json:"nodetarfile"`
    Uipath 			string `json:"uipath"`
	Uiconfpath		string `json:"uiconfpath"`
    Uitarfile 		string `json:"uitarfile"`
    Tmpfolder 		string `json:"tmpfolder"`
    Target 			[]string `json:"target"`
    Uifiles 		[]string `json:"uifiles"`
    Action 			string `json:"action"`
	Repourl 		string `json:"repourl"`
    Masterfiles 	[]string `json:"masterfiles"`
    Nodefiles 		[]string `json:"nodefiles"`
    Masterdb 		[]string `json:"masterdb"`
    Nodedb 			[]string `json:"nodedb"`
}

var file = "log.json"
var f *os.File
var config Config


func ReadConfig() Config{
	configStruct, err := os.Open("config.json")
	if err != nil {
		logs.Error(err)
	}

	defer configStruct.Close()
	b, err := ioutil.ReadAll(configStruct)
	if err != nil {
		logs.Error(err)
	}

	var localConfig Config
	json.Unmarshal([]byte(b), &localConfig)

	return localConfig
}

func UpdateJsonFile(newFile string, currentFile string){
	local, err := os.Open(currentFile)
	if err != nil {
		logs.Error(err)
	}

	remote, err := os.Open(newFile)
	if err != nil {
		logs.Error(err)
	}

	defer local.Close()
	defer remote.Close()

	b, err := ioutil.ReadAll(local)
	if err != nil {
		logs.Error(err)
	}

	c, err := ioutil.ReadAll(remote)
	if err != nil {
		logs.Error(err)
	}

	var localFile map[string]interface{}
	var remoteFile map[string]interface{}
	json.Unmarshal([]byte(b), &localFile)
	json.Unmarshal([]byte(c), &remoteFile)

	CompareJSONFile(localFile, remoteFile)

    LinesOutput, _ := json.Marshal(localFile)
	var out bytes.Buffer
	json.Indent(&out, LinesOutput, "","\t")
	ioutil.WriteFile(currentFile, out.Bytes(), 0644)

	return
}

func UpdateDBFile(currentDB string, newDB string){
	outRemote,err := exec.Command("sqlite3", newDB, ".table").Output()
	if err != nil {	logs.Error("UpdateDBFile outRemote: "+err.Error())}
	outLocal,err := exec.Command("sqlite3", currentDB, ".table").Output()
	if err != nil {	logs.Error("UpdateDBFile outLocal: "+err.Error())}

	re := regexp.MustCompile(`\s+`)
	outputRemote := re.ReplaceAllString(string(outRemote), "\n")
	outputLocal := re.ReplaceAllString(string(outLocal), "\n")
	splitLocal := strings.Split(outputLocal, "\n")
	splitRemote := strings.Split(outputRemote, "\n")

	var exists bool
	for w := range splitRemote {
		exists = false
		for z := range splitLocal {
			if splitRemote[w] == splitLocal[z] {
				exists = true
			}
		}
		if !exists{
			createTable,err := exec.Command("sqlite3", newDB, ".schema "+splitRemote[w]).Output()
			if err != nil {	logs.Error("UpdateDBFile Error Create table: "+err.Error())}
			database, err := sql.Open("sqlite3", currentDB)
			if err != nil {	logs.Error("UpdateDBFile Error Open table: "+err.Error())}
			statement, err := database.Prepare(string(createTable))
			if err != nil {	logs.Error("UpdateDBFile Error Prepare table: "+err.Error())}
			defer database.Close()
    		statement.Exec()
		}
	}
}

func Logger(data map[string]string){
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logs.Error(err)
		return
	}
	defer f.Close()

	LinesOutput, _ := json.Marshal(data)

	_,err = f.WriteString(string(LinesOutput)+"\n")
	if err != nil {
		logs.Error(err)
		return
	}

	return
}

func GetNewSoftware(service string)(err error){
	var url string
	var tarfile string
	switch service{
	case "owlhmaster":
		tarfile = config.Mastertarfile
		url = config.Repourl+tarfile
	case "owlhnode":
		tarfile = config.Nodetarfile
		url = config.Repourl+tarfile
	case "owlhui":
		tarfile = config.Uitarfile
		url = config.Repourl+tarfile
	default:
		return errors.New("UNKNOWN service to download GetNewSoftware")
	}
	
	err = DownloadFile(config.Tmpfolder+tarfile, url)
	if err != nil {	logs.Error("Error GetNewSoftware: "+err.Error()); return err }
	err = ExtractTarGz(config.Tmpfolder+tarfile, config.Tmpfolder+service)
	if err != nil {	logs.Error("Error GetNewSoftware: "+err.Error()); return err }

	return nil
}

func CopyBinary(service string)(err error){
	binFileSrc := config.Tmpfolder+service+"/"+service
	var binFileDst string
	switch service{
	case "owlhmaster":
		binFileDst = config.Masterbinpath+service
		err = os.MkdirAll(config.Masterbinpath, 0755)
		if err != nil {	logs.Error("CopyBinary MkDirAll error creating folder for Master: "+err.Error()) }
	case "owlhnode":
		binFileDst = config.Nodebinpath+service
		err = os.MkdirAll(config.Nodebinpath, 0755)
		if err != nil {	logs.Error("CopyBinary MkDirAll error creating folder for Node: "+err.Error()) }
	default:
		return errors.New("UNKNOWN service to download CopyBinary")
	}



	err = CopyFiles(binFileSrc, binFileDst)	
	if err != nil {	logs.Error("CopyBinary Error copy files: "+err.Error()); return err}
	
	return err
}

func UpdateTxtFile(src string, dst string)(err error){
	local, err := os.Open(src)
	if err != nil {logs.Error(err); return err}
	
	remote, err := os.Open(dst)
	if err != nil {logs.Error("Error opennign file for read UpdateTxtFile: "+err.Error()); return err}
	remoteWR, err := os.OpenFile(dst, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {logs.Error("Error opennign file for append UpdateTxtFile: "+err.Error()); return err}

	defer local.Close()
	defer remote.Close()
	defer remoteWR.Close()

	scannerSRC := bufio.NewScanner(local)
	scannerDST := bufio.NewScanner(remote)
	
	var totalLine []string
	dstLine :=make(map[string]string)

	for scannerDST.Scan() {
		dLine := strings.Split(scannerDST.Text(), " ")
		dstLine[dLine[0]] = scannerDST.Text()
	}

	for scannerSRC.Scan() {
		srcLine := strings.Split(scannerSRC.Text(), " ")
		if _, ok := dstLine[srcLine[0]]; !ok {
			totalLine = append(totalLine, scannerSRC.Text())
		}
	}
	
	for x := range totalLine {
		if _, err = remoteWR.WriteString("\n"+totalLine[x]); err != nil {
			logs.Error("Error writting to dst file: "+err.Error())
			return err
		}
	}
	return err
}

func UpdateFiles(service string)(err error){
	switch service{
	case "owlhmaster":
		for w := range config.Masterfiles{
			if _, err := os.Stat(config.Masterconfpath+config.Masterfiles[w]); os.IsNotExist(err) {				
				err = CopyFiles(config.Tmpfolder+service+"/conf/"+config.Masterfiles[w], config.Masterconfpath+config.Masterfiles[w])	
				if err != nil {	logs.Error("UpdateFiles Error copy files for master: "+err.Error()); return err}
			}else{
				if config.Masterfiles[w] == "app.conf"{
					logs.Debug("app.conf owlhmaster")
					err = UpdateTxtFile(config.Tmpfolder+service+"/conf/"+config.Masterfiles[w], config.Masterconfpath+config.Masterfiles[w])
					if err != nil {	logs.Error("UpdateTxtFile Error copy files for master: "+err.Error()); return err}
				}else{
					UpdateJsonFile(config.Tmpfolder+service+"/conf/"+config.Masterfiles[w], config.Masterconfpath+config.Masterfiles[w])
				}
			}
		}
		err = CopyFiles(config.Tmpfolder+"current.version", config.Masterconfpath+"current.version")
		if err != nil {	logs.Error("UpdateFiles Error CopyFiles for assign current current.version file: "+err.Error()); return err}

	case "owlhnode":
		for w := range config.Nodefiles{
			if _, err := os.Stat(config.Nodeconfpath+config.Nodefiles[w]); os.IsNotExist(err) {				
				err = CopyFiles(config.Tmpfolder+service+"/conf/"+config.Nodefiles[w], config.Nodeconfpath+config.Nodefiles[w])	
				if err != nil {	logs.Error("UpdateFiles Error copy files for Node: "+err.Error()); return err}
			}else{

				if config.Nodefiles[w] == "app.conf"{
					err = UpdateTxtFile(config.Tmpfolder+service+"/conf/"+config.Nodefiles[w], config.Nodeconfpath+config.Nodefiles[w])
					if err != nil {	logs.Error("UpdateTxtFile Error copy files for Node: "+err.Error()); return err}
				}else{
					UpdateJsonFile(config.Tmpfolder+service+"/conf/"+config.Nodefiles[w], config.Nodeconfpath+config.Nodefiles[w])
				}
			}
		}
		err = CopyFiles(config.Tmpfolder+"current.version", config.Nodeconfpath+"current.version")
		if err != nil {	logs.Error("UpdateFiles Error CopyFiles for assign current current.version file: "+err.Error()); return err}
	default:
		return errors.New("UNKNOWN service to download UpdateFiles")
	}

	return nil
}

func UpdateDb(service string)(err error){
	switch service{
	case "owlhmaster":
		for w := range config.Masterdb{
			if _, err := os.Stat(config.Masterconfpath+config.Masterdb[w]); os.IsNotExist(err) {				
				err = CopyFiles(config.Tmpfolder+service+"/conf/"+config.Masterdb[w], config.Masterconfpath+config.Masterdb[w])	
				if err != nil {	logs.Error("UpdateDb Error copy files for master: "+err.Error()); return err}
			}else{
				UpdateDBFile(config.Masterconfpath+config.Masterdb[w], config.Tmpfolder+service+"/conf/"+config.Masterdb[w])
			}
		}
	case "owlhnode":
		for w := range config.Nodedb{
			if _, err := os.Stat(config.Masterconfpath+config.Masterdb[w]); os.IsNotExist(err) {				
				err = CopyFiles(config.Tmpfolder+service+"/conf/"+config.Nodedb[w], config.Nodeconfpath+config.Nodedb[w])	
				if err != nil {	logs.Error("UpdateDb Error copy files for node: "+err.Error()); return err}
			}else{
				UpdateDBFile(config.Nodeconfpath+config.Nodedb[w], config.Tmpfolder+service+"/conf/"+config.Nodedb[w])
			}
		}
	default:
		return errors.New("UNKNOWN service to download UpdateDb")
	}

	return nil
}

func StartService(service string)(err error){
	if service == "owlhui" {
		if _, err := os.Stat("/etc/systemd/system/"); !os.IsNotExist(err) {
			logs.Info(service+" systemd starting...")
			_, err := exec.Command("bash","-c","systemctl restart httpd").Output()
			return err
		}else if _, err := os.Stat("/etc/init.d/"+service); !os.IsNotExist(err) {
			logs.Info(service+" systemV starting...")
			_, err := exec.Command("bash","-c","service httpd restart").Output()
			return err
		}
		return err
	}
	if _, err := os.Stat("/etc/systemd/system/"+service+".service"); !os.IsNotExist(err) {
		logs.Info(service+" systemd starting...")
		_, err := exec.Command("bash","-c","systemctl start "+service).Output()
		return err
	}else if _, err := os.Stat("/etc/init.d/"+service); !os.IsNotExist(err) {
		logs.Info(service+" systemV starting...")
		_, err := exec.Command("bash","-c","service "+service+" start").Output()
		return err
	}

	logs.Info(service+" alone")
	
	logs.Info("I can't start the service...")
	// out, err := exec.Command("bash","-c","kill -9 $(pidof "+service+")").Output()

	return nil
}

func StopService(service string) error{
	if _, err := os.Stat("/etc/systemd/system/"+service+".service"); !os.IsNotExist(err) {
		logs.Info(service+" systemd stopping...")
		_, err := exec.Command("bash","-c","systemctl stop "+service).Output()
		return err
	}else if _, err := os.Stat("/etc/init.d/"+service); !os.IsNotExist(err) {
		logs.Info(service+" systemV stopping...")
		_, err := exec.Command("bash","-c","service "+service+" stop").Output()
		return err
	}

	logs.Info(service+" alone")
	_, err := exec.Command("bash","-c","kill -9 $(pidof "+service+")").Output()
	return err
}

func BackupUiConf()(err error){
	for x := range config.Uifiles{
		err = CopyFiles(config.Uiconfpath+config.Uifiles[x], config.Tmpfolder+config.Uifiles[x]+".bck")
		if err != nil {	logs.Error("BackupUiConf Error CopyFiles for make a backup: "+err.Error()); return err}
	}
	return nil
}

func RestoreBackups()(err error){
	for x := range config.Uifiles{
		err = CopyFiles(config.Tmpfolder+config.Uifiles[x]+".bck", config.Uiconfpath+config.Uifiles[x])
		if err != nil {	logs.Error("BackupUiConf Error CopyFiles for make a backup: "+err.Error()); return err}
	}
		
	return nil
}



func CopyServiceFiles(service string)(err error){
	stype := systemType()
	stype = "systemV"
	switch service {
		case "owlhmaster":
			if stype == "systemd"{
				cmd := config.Masterbinpath+"defaults/services/systemd/owlhmaster.sh"
				_, err = exec.Command("bash","-c",cmd).Output()
				if err != nil {	logs.Error("CopyServiceFiles systemV ERROR: "+err.Error()); return err}
			}
			if stype == "systemV"{
				cmd := config.Masterbinpath+"defaults/services/systemV/owlhmaster.sh"
				_, err = exec.Command("bash","-c",cmd).Output()
				if err != nil {	logs.Error("CopyServiceFiles systemV ERROR: "+err.Error()); return err}
			}
		case "owlhnode":
			if stype == "systemd"{
				cmd := config.Masterbinpath+"defaults/services/systemd/owlhnode.sh"
				_, err = exec.Command("bash","-c",cmd).Output()
				if err != nil {	logs.Error("CopyServiceFiles systemV ERROR: "+err.Error()); return err}
			}
			if stype == "systemV"{
				cmd := config.Masterbinpath+"defaults/services/systemV/owlhnode.sh"
				_, err = exec.Command("bash","-c",cmd).Output()
				if err != nil {	logs.Error("CopyServiceFiles systemV ERROR: "+err.Error()); return err}
			}
		case "owlhui":
			cmd := config.Masterbinpath+"defaults/services/httpd/owlhui.sh"
			_, err = exec.Command("bash","-c",cmd).Output()
			if err != nil {	logs.Error("CopyServiceFiles systemV ERROR: "+err.Error()); return err}
		default:
			logs.Error("No service")
	}
	return nil
}

func ManageMaster(){
	var err error
	isError := false
	sessionLog := make(map[string]string)
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	sessionLog["date"] = currentTime
	service := "owlhmaster"
	switch config.Action {
	case "install":
		logs.Info("New Install for Master")
		sessionLog["status"] ="New Install for Master"
		Logger(sessionLog)

		logs.Info("Downloading New Software")
		err = GetNewSoftware(service)
		if err != nil {	logs.Error("ManageMaster Error INSTALL GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error getting new software for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		logs.Info("ManageMaster Stopping the service")
		err = StopService(service)
		if err != nil {	logs.Error("ManageMaster Error INSTALL StopService: "+err.Error()); sessionLog["status"] = "Error stopping service for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		logs.Info("ManageMaster Copying files from download")
		err = CopyBinary(service)
		if err != nil {	logs.Error("ManageMaster Error INSTALL CopyBinary: "+err.Error()); sessionLog["status"] = "Error copying binary for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		err = FullCopyDir(config.Tmpfolder+service+"/conf/", config.Masterconfpath)
		if err != nil {	logs.Error("FullCopyDir Error INSTALL Master: "+err.Error()); sessionLog["status"] = "Error copying full directory for Master: "+err.Error(); Logger(sessionLog); isError=true}

		err = FullCopyDir(config.Tmpfolder+service+"/defaults/", config.Masterconfpath+"defaults/")
		if err != nil {	logs.Error("FullCopyDir Error INSTALL Master: "+err.Error()); sessionLog["status"] = "Error copying full directory for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		// logs.Info("ManageMaster Installing service...")
		// err = CopyServiceFiles(service)
		// if err != nil {	logs.Error("CopyServiceFiles Error INSTALL Master: "+err.Error()); sessionLog["status"] = "Error copying service files for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		logs.Info("ManageMaster Copying current.version...")
		err = CopyFiles(config.Tmpfolder+"current.version", config.Masterconfpath+"current.version")
		if err != nil {	logs.Error("ManageMaster back up Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error copying files for Master: "+err.Error(); Logger(sessionLog); isError=true}
		
		logs.Info("ManageMaster Launching service...")
		err = StartService(service)
		if err != nil {	logs.Error("ManageMaster Error INSTALL StartService: "+err.Error()); sessionLog["status"] = "Error launching service for Master: "+err.Error(); Logger(sessionLog); isError=true}
				
		logs.Info("ManageMaster Done!")
		if isError {sessionLog["status"] = "ManageMaster installed with errors..."}else{sessionLog["status"] = "ManageMaster installed done!"}
		Logger(sessionLog)

	case "update":
		sessionLog["status"] ="Update for Master"
		Logger(sessionLog)
		needsUpdate,_ := CheckVersion(config.Masterconfpath)
		// if err != nil {	logs.Error("ManageMaster Error UPDATING needsUpdate: "+err.Error()); sessionLog["status"] = "Error checking version for Master: "+err.Error(); Logger(sessionLog); isError=true}
		if needsUpdate {

			err = GetNewSoftware(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error getting new software for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = StopService(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING StopService: "+err.Error()); sessionLog["status"] = "Error stopping service for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = CopyBinary(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING CopyBinary: "+err.Error()); sessionLog["status"] = "Error copying binary for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = UpdateFiles(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING UpdateFiles: "+err.Error()); sessionLog["status"] = "Error updating files for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = UpdateDb(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING UpdateDb: "+err.Error()); sessionLog["status"] = "Error updating DB for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = CopyFiles(config.Tmpfolder+"current.version", config.Masterconfpath+"current.version")
			if err != nil {	logs.Error("ManageMaster Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error copying files for Master: "+err.Error(); Logger(sessionLog); isError=true}
			err = StartService(service)
			if err != nil {	logs.Error("ManageMaster Error UPDATING StartService: "+err.Error()); sessionLog["status"] = "Error starting service for Master: "+err.Error(); Logger(sessionLog); isError=true}
		}else{
			logs.Info("Up to date")
			if isError{sessionLog["status"] = "ManageMaster updated with errors..."}else{sessionLog["status"] = "ManageMaster updated done!"}
			Logger(sessionLog)
		}
	default:
		logs.Info("UNKNOWN Action ManageMaster")
		sessionLog["status"] = "UNKNOWN Action ManageMaster"
		Logger(sessionLog)
	}
}
func ManageNode(){
	isError := false
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	sessionLog := make(map[string]string)
	sessionLog["date"] = currentTime
	var err error
	service := "owlhnode"
	switch config.Action {
	case "install":
		logs.Info("New Install for Node")
		sessionLog["status"] ="New Install for Node"
		Logger(sessionLog)

		logs.Info("Downloading New Software")
		err = GetNewSoftware(service)
		if err != nil {	logs.Error("ManageNode Error UPDATING GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error getting new software for Node: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageNode Stopping the service")
		err = StopService(service)
		if err != nil {	logs.Error("ManageNode Error UPDATING StopService: "+err.Error()); sessionLog["status"] = "Error Stopping service for Node: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageNode Copying files from download")
		err = CopyBinary(service)
		if err != nil {	logs.Error("ManageNode Error INSTALL CopyBinary: "+err.Error()); sessionLog["status"] = "Error copying binary for Node: "+err.Error(); Logger(sessionLog); isError=true}
		err = FullCopyDir(config.Tmpfolder+service+"/conf/", config.Nodeconfpath)
		if err != nil {	logs.Error("FullCopyDir Error INSTALL Node: "+err.Error()); sessionLog["status"] = "Error copying full conf directory for Node: "+err.Error(); Logger(sessionLog); isError=true}
		err = FullCopyDir(config.Tmpfolder+service+"/defaults/", config.Nodeconfpath+"defaults/")
		if err != nil {	logs.Error("FullCopyDir Error INSTALL Node: "+err.Error()); sessionLog["status"] = "Error copying full defaults directory for Node: "+err.Error(); Logger(sessionLog); isError=true}
		err = CopyFiles(config.Tmpfolder+"current.version", config.Nodeconfpath+"current.version")
		if err != nil {	logs.Error("ManageNode Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error Copying files for Node: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageNode Launching service...")
		err = StartService(service)
		if err != nil {	logs.Error("ManageNode Error UPDATING StartService: "+err.Error()); sessionLog["status"] = "Error launching service for Node: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageNode Done!")
		if isError {sessionLog["status"] = "ManageNode installed with errors..."}else{sessionLog["status"] = "ManageNode installed done!"}
		Logger(sessionLog)
	case "update":				
		sessionLog["status"] ="Update for Node"
		Logger(sessionLog)
		needsUpdate,_ := CheckVersion(config.Nodeconfpath)
		// if err != nil {	logs.Error("ManageNode Error UPDATING needsUpdate: "+err.Error()); sessionLog["status"] = "Error Checking version for Node: "+err.Error(); Logger(sessionLog); isError=true}
		if needsUpdate {

			err = GetNewSoftware(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error Getting new software for Node: "+err.Error(); Logger(sessionLog); isError=true}
			err = StopService(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING StopService: "+err.Error()); sessionLog["status"] ="Error Stopping service for Node:"+ err.Error(); Logger(sessionLog); isError=true}
			err = CopyBinary(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING CopyBinary: "+err.Error()); sessionLog["status"] = "Error Copying Binary for Node: "+err.Error(); Logger(sessionLog); isError=true}
			err = UpdateFiles(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING UpdateFiles: "+err.Error()); sessionLog["status"] = "Error updating files for Node: "+err.Error(); Logger(sessionLog); isError=true}
			err = UpdateDb(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING UpdateDb: "+err.Error()); sessionLog["status"] = "Error updating DB for Node: "+err.Error(); Logger(sessionLog); isError=true}
			err = CopyFiles(config.Tmpfolder+"current.version", config.Nodeconfpath+"current.version")
			if err != nil {	logs.Error("ManageNode BackupUiConf Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error Copying files for Node: "+err.Error(); Logger(sessionLog); isError=true}
			err = StartService(service)
			if err != nil {	logs.Error("ManageNode Error UPDATING StartService: "+err.Error()); sessionLog["status"] = "Error launching service error for Node:"+err.Error(); Logger(sessionLog); isError=true}
			if isError {sessionLog["status"] = "ManageNode updated with errors..."}else{sessionLog["status"] = "ManageNode updated done!"}
			Logger(sessionLog)
		}else{
			logs.Info("Up to date")
			sessionLog["status"] ="Node is up to date"
			Logger(sessionLog)
		}
	default:
		logs.Info("UNKNOWN Action ManageNode")
		sessionLog["status"] ="UNKNOWN Action ManageNode"
		Logger(sessionLog)
	}
}
func ManageUI(){
	isError := false
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	sessionLog := make(map[string]string)
	sessionLog["date"] = currentTime

	var err error
	service := "owlhui"
	switch config.Action {
	case "install":
		logs.Info("New Install for UI")
		sessionLog["status"] ="New Install for UI"
		Logger(sessionLog)	

		logs.Info("Downloading New Software")
		err = GetNewSoftware(service)
		if err != nil {	logs.Error("ManageUI Error UPDATING GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error getting new software for UI: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageUI Copying files from download")
		err = FullCopyDir(config.Tmpfolder+service, config.Uipath)
		if err != nil {	logs.Error("ManageUI FullCopyDir Error INSTALL Node: "+err.Error()); sessionLog["status"] = "Error copying full directory for UI: "+err.Error(); Logger(sessionLog); isError=true}
		logs.Info("ManageUI Launching service...")
		err = CopyFiles(config.Tmpfolder+"current.version", config.Uiconfpath+"current.version")
		if err != nil {	logs.Error("ManageUI BackupUiConf Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error copying files for UI: "+err.Error(); Logger(sessionLog); isError=true}
		err = StartService(service)
		if err != nil {	logs.Error("ManageUI Error UPDATING StartService: "+err.Error()); sessionLog["status"] = "Error starting service for UI: "+err.Error(); Logger(sessionLog); isError=true}
		if isError {sessionLog["status"] = "ManageUI updated with errors..."}else{sessionLog["status"] = "ManageUI updated done!"}
		Logger(sessionLog)
		logs.Info("ManageUI Done!")
	case "update":
		sessionLog["status"] ="New Update for UI"
		Logger(sessionLog)	

		needsUpdate,_ := CheckVersion(config.Uiconfpath)
		// if err != nil {	logs.Error("ManageUI Error UPDATING needsUpdate: "+err.Error()); sessionLog["status"] = "Error checking version for UI: "+err.Error(); Logger(sessionLog); isError=true}
		if needsUpdate {

			err = GetNewSoftware(service)
			if err != nil {	logs.Error("ManageUI Error UPDATING GetNewSoftware: "+err.Error()); sessionLog["status"] = "Error Getting new software for UI: "+err.Error(); Logger(sessionLog); isError=true}
			err = BackupUiConf()			
			if err != nil {	logs.Error("ManageUI Error UPDATING ui.conf backup: "+err.Error()); sessionLog["status"] = "Error backing up configuration file software for UI: "+err.Error(); Logger(sessionLog); isError=true}
			err = FullCopyDir(config.Tmpfolder+service, config.Uipath)
			if err != nil {	logs.Error("ManageUI CopyAllUiFiles Error copying new elements to directory: "+err.Error()); sessionLog["status"] = "Error copying full directory for UI: "+err.Error(); Logger(sessionLog); isError=true}
			err = CopyFiles(config.Tmpfolder+"current.version", config.Uiconfpath+"current.version")
			if err != nil {	logs.Error("ManageUI BackupUiConf Error CopyFiles for assign current current.version file: "+err.Error()); sessionLog["status"] = "Error copying files for UI: "+err.Error(); Logger(sessionLog); isError=true}
			err = RestoreBackups()			
			if err != nil {	logs.Error("ManageUI Error UPDATING RestoreBackups: "+err.Error()); sessionLog["status"] = "Error restoring backups for UI: "+err.Error(); Logger(sessionLog); isError=true}
			if isError {sessionLog["status"] = "ManageUI updated with errors..."}else{sessionLog["status"] = "ManageUI updated done!"}
			Logger(sessionLog)
		}else{
			logs.Info("Up to date")
			sessionLog["status"] ="UI is up to date"
			Logger(sessionLog)
		}
	default:
		logs.Info("UNKNOWN Action ManageUI")
		sessionLog["status"] ="UNKNOWN Action ManageUI"
		Logger(sessionLog)
	}
}

func main() {
	var err error
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	sessionLog := make(map[string]string)
	sessionLog["date"] = currentTime
	sessionLog["status"] = "--- Start Updater ---"
	Logger(sessionLog)

	//Read Struct
	config = ReadConfig()
	//Download current version
	DownloadCurrentVersion()

	logs.Info(config.Target)
	for w := range config.Target {
		switch config.Target[w] {
		case "master":
			if _, err = os.Stat(config.Masterbinpath); !os.IsNotExist(err) {
				ManageMaster()
			}else if config.Action == "install"{
				ManageMaster()				
			}
		case "node":
			if _, err = os.Stat(config.Nodebinpath); !os.IsNotExist(err) {
				ManageNode()
			}else if config.Action == "install"{
				ManageNode()
			}
		case "ui":
			if _, err = os.Stat(config.Uipath); !os.IsNotExist(err) {				
				ManageUI()
			}else if config.Action == "install"{
				ManageUI()
			}
		default:
			logs.Info("UNKNOWN Target at Main()")
			sessionLog["status"] = "UNKNOWN Target at Main()"
			Logger(sessionLog)
		}
	}

	sessionLog["status"] = "Removing /tmp data"
	Logger(sessionLog)
	for w := range config.Target {

		switch config.Target[w] {
		case "master":
			err = RemoveDownloadedFiles("owlhmaster")
			if err != nil {	logs.Error("ManageMaster Error INSTALL RemoveDownloadedFiles: "+err.Error()); sessionLog["status"] = "Error removing /tmp files for Master: "+err.Error(); Logger(sessionLog)}
		case "node":
			err = RemoveDownloadedFiles("owlhnode")
			if err != nil {	logs.Error("ManageNode Error INSTALL RemoveDownloadedFiles: "+err.Error()); sessionLog["status"] = "Error removing /tmp files for Node: "+err.Error(); Logger(sessionLog)}
		case "ui":
			err = RemoveDownloadedFiles("owlhui")
			if err != nil {	logs.Error("ManageUi Error INSTALL RemoveDownloadedFiles: "+err.Error()); sessionLog["status"] = "Error removing /tmp files for UI: "+err.Error(); Logger(sessionLog)}
		default:
			logs.Info("UNKNOWN Target at Main()")
			sessionLog["status"] = "UNKNOWN Target at Main()"
			Logger(sessionLog)
		}
	}
	
	currentTime = time.Now().Format("2006-01-02 15:04:05")
	sessionLog = make(map[string]string)
	sessionLog["date"] = currentTime
	sessionLog["status"] = "--- End Updater ---"
	Logger(sessionLog)

	return
}