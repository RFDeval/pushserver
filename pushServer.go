// pushServer.go
package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Unknwon/goconfig"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type AppConfig struct {
	ApolloHost     string
	ApolloPort     string
	ApolloUser     string
	ApolloPassword string
	ApolloTopics   map[string]string
	pushServerName string
	PushAddress    string

	smtpUser     string
	smtpPassword string
	smtpHost     string
	smtpTo       string
	smtpSubject  string
}

var gAppConfig AppConfig

func readConfig() {
	cfg, err := goconfig.LoadConfigFile("conf.ini")
	if err != nil {
		log.Fatalf("读取配置文件失败：%s", err)
	}
	gAppConfig.ApolloHost, _ = cfg.GetValue("apollo", "host")
	gAppConfig.ApolloPort, _ = cfg.GetValue("apollo", "port")
	gAppConfig.ApolloUser, _ = cfg.GetValue("apollo", "user")
	gAppConfig.ApolloPassword, _ = cfg.GetValue("apollo", "password")
	gAppConfig.ApolloTopics, err = cfg.GetSection("topic")
	if err != nil {
		log.Fatalf("无法获取配置文件的分区：%s", err)
	}
	gAppConfig.pushServerName, _ = cfg.GetValue("pushServer", "name")
	gAppConfig.PushAddress, _ = cfg.GetValue("pushServer", "address")

	gAppConfig.smtpUser, _ = cfg.GetValue("smtp", "user")
	gAppConfig.smtpPassword, _ = cfg.GetValue("smtp", "password")
	gAppConfig.smtpHost, _ = cfg.GetValue("smtp", "host")
	gAppConfig.smtpTo, _ = cfg.GetValue("smtp", "to")
	gAppConfig.smtpSubject, _ = cfg.GetValue("smtp", "subject")

}

func main() {

	readConfig()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)
	done := make(chan struct{})

	log.Printf("servers starting......")

	go h.run()
	go websocketListen(done)
	go InitApollo(done)

	<-sigc
	for i := 0; i < 2; i++ {
		<-done
	}

	log.Printf("all bye")

}

var f MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	h.broadcast <- msg.Payload()
}

var myOnConnectHandler MQTT.OnConnectHandler = func(client MQTT.Client) {
	log.Printf("Apollo connected!")
	for _, v := range gAppConfig.ApolloTopics {
		if token := client.Subscribe(v, 0, nil); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
		} else {
			fmt.Println("Subscribe:", v)
		}
	}
}

var myConnectionLostHandler MQTT.ConnectionLostHandler = func(client MQTT.Client, e error) {
	fmt.Printf("error: %s\n", e)
	Notice("Apollo lost")
}

func InitApollo(done chan struct{}) {
	opts := MQTT.NewClientOptions().AddBroker("tcp://" + gAppConfig.ApolloHost + ":" + gAppConfig.ApolloPort)
	opts.SetClientID(gAppConfig.pushServerName)
	opts.SetUsername(gAppConfig.ApolloUser)
	opts.SetPassword(gAppConfig.ApolloPassword)
	opts.SetDefaultPublishHandler(f)
	opts.SetOnConnectHandler(myOnConnectHandler)
	opts.SetConnectionLostHandler(myConnectionLostHandler)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Second)

	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	done <- struct{}{}
	for _, v := range gAppConfig.ApolloTopics {
		if token := c.Unsubscribe(v); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
		} else {
			fmt.Println("Unsubscribe:", v)
		}
	}
	c.Disconnect(250)
}

func Notice(msg string) {
	body := "<html><body><h1>" + msg + "</h1></body></html>"
	err := SendToMail(gAppConfig.smtpUser, gAppConfig.smtpPassword, gAppConfig.smtpHost, gAppConfig.smtpTo, gAppConfig.smtpSubject, body, "html")
	if err != nil {
		fmt.Println("Send mail error!", err)
	} else {
		fmt.Println("Send mail success!")
	}
}

func SendToMail(user, password, host, to, subject, body, mailtype string) error {
	hostport := strings.Split(host, ":")
	auth := smtp.PlainAuth("", user, password, hostport[0])
	var content_type string
	if mailtype == "html" {
		content_type = "Content-Type: text/" + mailtype + "; charset=UTF-8"
	} else {
		content_type = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	msg := []byte("To: " + to + "\r\nFrom: " + user + "\r\nSubject: " + subject + "\r\n" + content_type + "\r\n\r\n" + body)
	send_to := strings.Split(to, ";")
	err := smtp.SendMail(host, auth, user, send_to, msg)
	return err
}
