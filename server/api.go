// replication-manager - Replication Manager Monitoring and CLI for MariaDB and MySQL
// Copyright 2017-2021 SIGNAL18 CLOUD SAS
// Author: Stephane Varoqui  <svaroqui@gmail.com>
// License: GNU General Public License, version 3. Redistribution/Reuse of this code is permitted under the GNU v3 license, as an additional term ALL code must carry the original Author(s) credit in comment form.
// See LICENSE in this directory for the integral text.

package server

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/codegangsta/negroni"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/gorilla/mux"
	"github.com/signal18/replication-manager/cert"
	"github.com/signal18/replication-manager/cluster"
	"github.com/signal18/replication-manager/regtest"
	"github.com/signal18/replication-manager/share"
	"github.com/signal18/replication-manager/utils/githelper"
)

//RSA KEYS AND INITIALISATION

var signingKey, verificationKey []byte
var apiPass string
var apiUser string

func (repman *ReplicationManager) initKeys() {
	var (
		err         error
		privKey     *rsa.PrivateKey
		pubKey      *rsa.PublicKey
		pubKeyBytes []byte
	)

	privKey, err = rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		log.Fatal("Error generating private key")
	}
	pubKey = &privKey.PublicKey //hmm, this is stdlib manner...

	// Create signingKey from privKey
	// prepare PEM block
	var privPEMBlock = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey), // serialize private key bytes
	}
	// serialize pem
	privKeyPEMBuffer := new(bytes.Buffer)
	pem.Encode(privKeyPEMBuffer, privPEMBlock)
	//done
	signingKey = privKeyPEMBuffer.Bytes()

	//fmt.Println(string(signingKey))

	// create verificationKey from pubKey. Also in PEM-format
	pubKeyBytes, err = x509.MarshalPKIXPublicKey(pubKey) //serialize key bytes
	if err != nil {
		// heh, fatality
		log.Fatal("Error marshalling public key")
	}

	var pubPEMBlock = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	}
	// serialize pem
	pubKeyPEMBuffer := new(bytes.Buffer)
	pem.Encode(pubKeyPEMBuffer, pubPEMBlock)
	// done
	verificationKey = pubKeyPEMBuffer.Bytes()

	//	fmt.Println(string(verificationKey))
}

//STRUCT DEFINITIONS

type userCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type apiresponse struct {
	Data string `json:"data"`
}

type token struct {
	Token string `json:"token"`
}

func (repman *ReplicationManager) DashboardFSHandler() http.Handler {
	sub, err := fs.Sub(share.EmbededDbModuleFS, "dashboard")
	if err != nil {
		panic(err)
	}

	return http.FileServer(http.FS(sub))
}

func (repman *ReplicationManager) DashboardFSHandlerApp() http.Handler {
	sub, err := fs.Sub(share.EmbededDbModuleFS, "dashboard/app.html")
	if err != nil {
		panic(err)
	}

	return http.FileServer(http.FS(sub))
}

func (repman *ReplicationManager) rootHandler(w http.ResponseWriter, r *http.Request) {
	html, err := share.EmbededDbModuleFS.ReadFile("dashboard/app.html")
	if err != nil {
		log.Printf("rootHandler read error : %s", err)
	}
	w.Write(html)
}

func (repman *ReplicationManager) apiserver() {
	repman.initKeys()
	//PUBLIC ENDPOINTS
	router := mux.NewRouter()
	//router.HandleFunc("/", repman.handlerApp)
	// page to view which does not need authorization
	if repman.Conf.Test {
		router.HandleFunc("/", repman.handlerApp)
		router.PathPrefix("/static/").Handler(http.FileServer(http.Dir(repman.Conf.HttpRoot)))
		router.PathPrefix("/app/").Handler(http.FileServer(http.Dir(repman.Conf.HttpRoot)))
	} else {
		router.HandleFunc("/", repman.rootHandler)
		router.PathPrefix("/static/").Handler(repman.DashboardFSHandler())
		router.PathPrefix("/app/").Handler(repman.DashboardFSHandler())
	}

	router.HandleFunc("/api/login", repman.loginHandler)
	//router.Handle("/api", v3.NewHandler("My API", "/swagger.json", "/api"))

	router.Handle("/api/auth/callback", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxAuthCallback)),
	))
	router.Handle("/api/clusters", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxClusters)),
	))
	router.Handle("/api/prometheus", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxPrometheus)),
	))
	router.Handle("/api/status", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxStatus)),
	))
	router.Handle("/api/timeout", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxTimeout)),
	))
	router.Handle("/api/repocomp/current", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerRepoComp)),
	))
	//UNPROTECTED ENDPOINTS FOR SETTINGS
	router.Handle("/api/monitor", negroni.New(
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxReplicationManager)),
	))
	//PROTECTED ENDPOINTS FOR SETTINGS
	router.Handle("/api/monitor", negroni.New(
		negroni.HandlerFunc(repman.validateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxReplicationManager)),
	))

	router.Handle("/api/monitor/actions/adduser/{userName}", negroni.New(
		negroni.HandlerFunc(repman.validateTokenMiddleware),
		negroni.Wrap(http.HandlerFunc(repman.handlerMuxAddUser)),
	))

	repman.apiDatabaseUnprotectedHandler(router)
	repman.apiDatabaseProtectedHandler(router)
	repman.apiClusterUnprotectedHandler(router)
	repman.apiClusterProtectedHandler(router)
	repman.apiProxyProtectedHandler(router)

	var err error

	tlsConfig := Repmanv3TLS{
		Enabled: false,
	}
	// Add default unsecure cert if not set
	if repman.Conf.MonitoringSSLCert == "" {
		host := repman.Conf.APIBind
		if host == "0.0.0.0" {
			host = "localhost," + host + ",127.0.0.1"
		}
		cert.Host = host
		cert.Organization = "Signal18 Replication-Manager"
		tmpKey, tmpCert, err := cert.GenerateTempKeyAndCert()
		if err != nil {
			log.Errorf("Cannot generate temporary Certificate and/or Key: %s", err)
		}
		log.Info("No TLS certificate provided using generated key (", tmpKey, ") and certificate (", tmpCert, ")")
		defer os.Remove(tmpKey)
		defer os.Remove(tmpCert)

		tlsConfig = Repmanv3TLS{
			Enabled:            true,
			CertificatePath:    tmpCert,
			CertificateKeyPath: tmpKey,
			SelfSigned:         true,
		}
	}

	if repman.Conf.MonitoringSSLCert != "" {
		log.Info("Starting HTTPS & JWT API on " + repman.Conf.APIBind + ":" + repman.Conf.APIPort)
		tlsConfig = Repmanv3TLS{
			Enabled:            true,
			CertificatePath:    repman.Conf.MonitoringSSLCert,
			CertificateKeyPath: repman.Conf.MonitoringSSLKey,
		}
	} else {
		log.Info("Starting HTTP & JWT API on " + repman.Conf.APIBind + ":" + repman.Conf.APIPort)
	}

	repman.SetV3Config(Repmanv3Config{
		Listen: Repmanv3ListenAddress{
			Address: repman.Conf.APIBind,
			Port:    repman.Conf.APIPort,
		},
		TLS: tlsConfig,
	})

	// pass the router to the V3 server that will multiplex the legacy API and the
	// new gRPC + JSON Gateway API.
	err = repman.StartServerV3(true, router)

	if err != nil {
		log.Errorf("JWT API can't start: %s", err)
	}

}

//////////////////////////////////////////
/////////////ENDPOINT HANDLERS////////////
/////////////////////////////////////////

func (repman *ReplicationManager) isValidRequest(r *http.Request) bool {

	_, err := request.ParseFromRequest(r, request.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
		vk, _ := jwt.ParseRSAPublicKeyFromPEM(verificationKey)
		return vk, nil
	})
	if err == nil {
		return true
	}
	return false
}

func (repman *ReplicationManager) IsValidClusterACL(r *http.Request, cluster *cluster.Cluster) bool {

	token, err := request.ParseFromRequest(r, request.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
		vk, _ := jwt.ParseRSAPublicKeyFromPEM(verificationKey)
		return vk, nil
	})
	if err == nil {
		claims := token.Claims.(jwt.MapClaims)
		userinfo := claims["CustomUserInfo"]
		mycutinfo := userinfo.(map[string]interface{})
		meuser := mycutinfo["Name"].(string)
		mepwd := mycutinfo["Password"].(string)
		_, ok := mycutinfo["profile"]

		if ok {
			if strings.Contains(mycutinfo["profile"].(string), repman.Conf.OAuthProvider) /*&& strings.Contains(mycutinfo["email_verified"]*/ {
				meuser = mycutinfo["email"].(string)
				return cluster.IsValidACL(meuser, mepwd, r.URL.Path, "oidc")
			}
		}
		return cluster.IsValidACL(meuser, mepwd, r.URL.Path, "password")
	}
	return false
}

func (repman *ReplicationManager) loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var user userCredentials

	//decode request into UserCredentials struct
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Error in request")
		return
	}
	if auth_try, ok := repman.UserAuthTry[user.Username]; ok {
		if auth_try.Try == 3 {
			if time.Now().Before(auth_try.Time.Add(3 * time.Minute)) {
				fmt.Println("Time until last auth try : " + time.Until(auth_try.Time).String())
				fmt.Println("3 authentication errors for the user " + user.Username + ", please try again in 3 minutes")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			} else {
				auth_try.Try = 1
				auth_try.Time = time.Now()
				repman.UserAuthTry[user.Username] = auth_try
			}
		} else {

			auth_try.Try += 1
			repman.UserAuthTry[user.Username] = auth_try
		}
	} else {
		var auth_try authTry
		auth_try.User = user.Username
		auth_try.Try = 1
		auth_try.Time = time.Now()
		repman.UserAuthTry[user.Username] = auth_try
	}

	for _, cluster := range repman.Clusters {
		//validate user credentials
		if cluster.IsValidACL(user.Username, user.Password, r.URL.Path, "oidc") {
			var auth_try authTry
			auth_try.Try = 1
			auth_try.Time = time.Now()
			repman.UserAuthTry[user.Username] = auth_try

			signer := jwt.New(jwt.SigningMethodRS256)
			claims := signer.Claims.(jwt.MapClaims)
			//set claims
			claims["iss"] = "https://api.replication-manager.signal18.io"
			claims["iat"] = time.Now().Unix()
			claims["exp"] = time.Now().Add(time.Hour * 48).Unix()
			claims["jti"] = "1" // should be user ID(?)
			claims["CustomUserInfo"] = struct {
				Name     string
				Role     string
				Password string
			}{user.Username, "Member", user.Password}
			signer.Claims = claims
			sk, _ := jwt.ParseRSAPrivateKeyFromPEM(signingKey)
			//sk, _ := jwt.ParseRSAPublicKeyFromPEM(signingKey)

			tokenString, err := signer.SignedString(sk)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, "Error while signing the token")
				log.Printf("Error signing token: %v\n", err)
			}

			//create a token instance using the token string

			specs := r.Header.Get("Accept")
			resp := token{tokenString}
			if strings.Contains(specs, "text/html") {
				w.Write([]byte(tokenString))
				return
			}

			repman.jsonResponse(resp, w)
			return
		}
	}

	w.WriteHeader(http.StatusForbidden)
	fmt.Println("Error logging in")
	fmt.Fprint(w, "Invalid credentials")
	return

	//create a rsa 256 signer

}

func (repman *ReplicationManager) handlerMuxAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	OAuthContext := context.Background()
	Provider, err := oidc.NewProvider(OAuthContext, repman.Conf.OAuthProvider)
	if err != nil {
		log.Printf("OAuth callback Failed to init oidc from gitlab:%s %v\n", repman.Conf.OAuthProvider, err)
		return
	}
	OAuthConfig := oauth2.Config{
		ClientID:     repman.Conf.OAuthClientID,
		ClientSecret: repman.Conf.GetDecryptedPassword("api-oauth-client-secret", repman.Conf.OAuthClientSecret),
		Endpoint:     Provider.Endpoint(),
		RedirectURL:  repman.Conf.APIPublicURL + "/api/auth/callback",
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "read_api", "api"},
	}
	log.Printf("OAuth oidc to gitlab: %v\n", OAuthConfig)
	oauth2Token, err := OAuthConfig.Exchange(OAuthContext, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	repman.OAuthAccessToken = oauth2Token

	userInfo, err := Provider.UserInfo(OAuthContext, oauth2.StaticTokenSource(oauth2Token))
	if err != nil {
		http.Error(w, "Failed to get userinfo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	r.Header.Get("Accept")

	for _, cluster := range repman.Clusters {
		//validate user credentials
		if cluster.IsValidACL(userInfo.Email, cluster.APIUsers[userInfo.Email].Password, r.URL.Path, "oidc") {
			apiuser := cluster.APIUsers[userInfo.Email]
			apiuser.GitToken = oauth2Token.AccessToken
			tmp := strings.Split(userInfo.Profile, "/")
			apiuser.GitUser = tmp[len(tmp)-1]
			cluster.APIUsers[userInfo.Email] = apiuser

			if cluster.Conf.Cloud18 {
				new_token, user_id := githelper.GetGitLabTokenOAuth(oauth2Token.AccessToken, cluster.Conf.LogGit)
				//vault_aut_url := vaulthelper.GetVaultOIDCAuth()
				//vaulthelper.GetVaultOIDCAuth()
				//http.Redirect(w, r, vault_aut_url, http.StatusSeeOther)
				if new_token != "" {
					//to create project for user if not exist
					path := cluster.Conf.Cloud18Domain + "/" + cluster.Conf.Cloud18SubDomain + "-" + cluster.Conf.Cloud18SubDomainZone
					name := cluster.Conf.Cloud18SubDomain + "-" + cluster.Conf.Cloud18SubDomainZone
					githelper.GitLabCreateProject(new_token, name, path, cluster.Conf.Cloud18Domain, user_id, cluster.Conf.LogGit)
					//to store new gitlab token
					cluster.Conf.GitUrl = repman.Conf.OAuthProvider + "/" + cluster.Conf.Cloud18Domain + "/" + cluster.Conf.Cloud18SubDomain + "-" + cluster.Conf.Cloud18SubDomainZone + ".git"
					cluster.Conf.GitUsername = tmp[len(tmp)-1]
					newSecret := cluster.Conf.Secrets["git-acces-token"]
					newSecret.OldValue = newSecret.Value
					newSecret.Value = new_token
					cluster.Conf.Secrets["git-acces-token"] = newSecret
					//cluster.Conf.GitAccesToken = tokenInfo.token
					cluster.Conf.CloneConfigFromGit(cluster.Conf.GitUrl, cluster.Conf.GitUsername, cluster.Conf.Secrets["git-acces-token"].Value, cluster.Conf.WorkingDir)
				} else {
					log.Printf("Failed to get token from gitlab: %v\n", err)
				}

			}

			signer := jwt.New(jwt.SigningMethodRS256)
			claims := signer.Claims.(jwt.MapClaims)
			//set claims
			claims["iss"] = "https://api.replication-manager.signal18.io"
			claims["iat"] = time.Now().Unix()
			claims["exp"] = time.Now().Add(time.Hour * 48).Unix()
			claims["jti"] = "1" // should be user ID(?)
			claims["CustomUserInfo"] = struct {
				Name     string
				Role     string
				Password string
			}{userInfo.Email, "Member", cluster.APIUsers[userInfo.Email].Password}
			password := cluster.APIUsers[userInfo.Email].Password
			signer.Claims = claims
			sk, _ := jwt.ParseRSAPrivateKeyFromPEM(signingKey)

			tokenString, err := signer.SignedString(sk)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, "Error while signing the token")
				log.Printf("Error signing token: %v\n", err)
			}
			//create a token instance using the token string
			specs := r.Header.Get("Accept")
			resp := token{tokenString}
			if strings.Contains(specs, "text/html") {
				http.Redirect(w, r, repman.Conf.APIPublicURL+"/#!/dashboard?token="+tokenString+"&user="+userInfo.Email+"&pass="+password, http.StatusTemporaryRedirect)
				return
			}
			repman.jsonResponse(resp, w)
			return
		}

	}

	w.WriteHeader(http.StatusForbidden)
	fmt.Println("Error logging in")
	fmt.Fprint(w, "Invalid credentials")
	return
}

//AUTH TOKEN VALIDATION

func (repman *ReplicationManager) handlerMuxReplicationManager(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	mycopy := repman
	var cl []string

	for _, cluster := range repman.Clusters {

		if repman.IsValidClusterACL(r, cluster) {
			cl = append(cl, cluster.Name)
		}
	}
	mycopy.ClusterList = cl
	e := json.NewEncoder(w)
	e.SetIndent("", "\t")
	err := e.Encode(mycopy)

	//err := e.Encode(repman)
	if err != nil {
		http.Error(w, "Encoding error", 500)
		return
	}

}

func (repman *ReplicationManager) handlerMuxAddUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	vars := mux.Vars(r)
	for _, cluster := range repman.Clusters {
		if repman.IsValidClusterACL(r, cluster) {
			cluster.AddUser(vars["userName"])
		}
	}

}

// swagger:route GET /api/clusters clusters
//
// This will show all the available clusters
//
//	Responses:
//	  200: clusters
func (repman *ReplicationManager) handlerMuxClusters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if repman.isValidRequest(r) {

		var clusters []*cluster.Cluster

		for _, cluster := range repman.Clusters {
			if repman.IsValidClusterACL(r, cluster) {
				clusters = append(clusters, cluster)
			}
		}

		sort.Sort(cluster.ClusterSorter(clusters))

		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		err := e.Encode(clusters)
		if err != nil {
			http.Error(w, "Encoding error", 500)
			return
		}
	} else {
		http.Error(w, "Unauthenticated", 401)
		return
	}
}

func (repman *ReplicationManager) validateTokenMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	//validate token
	token, err := request.ParseFromRequest(r, request.AuthorizationHeaderExtractor,
		func(token *jwt.Token) (interface{}, error) {
			vk, _ := jwt.ParseRSAPublicKeyFromPEM(verificationKey)
			return vk, nil
		})

	if err == nil {
		if token.Valid {
			next(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Token is not valid")
		}
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Unauthorised access to this resource"+err.Error())
	}
}

//HELPER FUNCTIONS

func (repman *ReplicationManager) jsonResponse(apiresponse interface{}, w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json, err := json.Marshal(apiresponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func (repman *ReplicationManager) handlerMuxClusterAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	vars := mux.Vars(r)
	repman.AddCluster(vars["clusterName"], "")

}

func (repman *ReplicationManager) handlerMuxClusterDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	vars := mux.Vars(r)
	repman.DeleteCluster(vars["clusterName"])

}

// swagger:operation GET /api/prometheus prometheus
// Returns the Prometheus metrics for all database instances on the server
// in the Prometheus text format
//
// ---
// produces:
//   - text/plain; version=0.0.4
//
// responses:
//
//	'200':
//	  description: Prometheus file format
//	  schema:
//	    type: string
//	  headers:
//	    Access-Control-Allow-Origin:
//	      type: string
func (repman *ReplicationManager) handlerMuxPrometheus(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Access-Control-Allow-Origin", "*")
	for _, cluster := range repman.Clusters {
		for _, server := range cluster.Servers {
			res := server.GetPrometheusMetrics()
			w.Write([]byte(res))
		}
	}
}

func (repman *ReplicationManager) handlerMuxClustersOld(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s := new(Settings)
	s.Clusters = repman.ClusterList
	regtest := new(regtest.RegTest)
	s.RegTests = regtest.GetTests()
	e := json.NewEncoder(w)
	e.SetIndent("", "\t")
	err := e.Encode(s)
	if err != nil {
		http.Error(w, "Encoding error", 500)
		return
	}
}

// The Status contains string value for the alive status.
// Possible values are: running, starting, errors
//
// swagger:response status
type StatusResponse struct {
	// Example: *
	AccessControlAllowOrigin string `json:"Access-Control-Allow-Origin"`
	// The status message
	// in: body
	Body struct {
		// Example: running
		// Example: starting
		// Example: errors
		Alive string `json:"alive"`
	}
}

// swagger:route GET /api/status status
//
// This will show the status of the cluster
//
//     Responses:
//       200: status

func (repman *ReplicationManager) handlerMuxStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if repman.isStarted {
		io.WriteString(w, `{"alive": "running"}`)
	} else {
		io.WriteString(w, `{"alive": "starting"}`)
	}
}

// swagger:route GET /api/timeout timeout
//
//     Responses:
//       200: status

func (repman *ReplicationManager) handlerMuxTimeout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	time.Sleep(1200 * time.Second)
	io.WriteString(w, `{"alive": "running"}`)
}

// swagger:route GET /api/heartbeat heartbeat
//
//     Responses:
//       200: heartbeat

func (repman *ReplicationManager) handlerMuxMonitorHeartbeat(w http.ResponseWriter, r *http.Request) {
	var send Heartbeat
	send.UUID = repman.UUID
	send.UID = repman.Conf.ArbitrationSasUniqueId
	send.Secret = repman.Conf.ArbitrationSasSecret
	send.Status = repman.Status
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if err := json.NewEncoder(w).Encode(send); err != nil {
		panic(err)
	}
}
