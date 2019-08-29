package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/mschneider82/go-smtp/smtpclient"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"gopkg.in/alecthomas/kingpin.v2"
)

var log = logrus.New()

func init() {
	customFormatter := new(prefixed.TextFormatter)
	//customFormatter := new(logrus.TextFormatter)
	customFormatter.DisableTimestamp = false
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.Formatter = customFormatter
	log.Level = logrus.InfoLevel
}

var defaultData = `Date: {{.Date}}
From: {{.From}}
To: {{.To}}
Subject: test {{.Date}}
Message-Id: {{.MsgID}}
X-Mailer: goaks github.com/mschneider82/goaks

This is a test mailing

`

type DataProperties struct {
	Date  string
	MsgID string
	From  string
	To    string
}

var (
	data             = kingpin.Flag("data", "set custom mailbody").Default(defaultData).String()
	dataFromFile     = kingpin.Flag("dataFromFile", "data from file").File()
	from             = kingpin.Flag("from", "from address").Default("from@example.com").String()
	to               = kingpin.Flag("to", "rcpt address").Default("to@example.com").Strings()
	server           = kingpin.Flag("server", "destination server").Default("127.0.0.1:25").String()
	helo             = kingpin.Flag("helo", "helo string").Default("example.com").String()
	extentDataFactor = kingpin.Flag("extentDataFactor", "1kb * factor").Default("0").Int()
)

func renderTemplate(data string) *bytes.Buffer {
	tmplData, err := template.New("data").Parse(data)
	if err != nil {
		log.WithError(err).Fatalf("")
	}

	bufData := &bytes.Buffer{}

	dataProperties := DataProperties{
		Date:  time.Now().Format(time.RFC1123Z),
		MsgID: fmt.Sprintf("<%s@goaks>", GenRandomID(12)),
		From:  *from,
		To:    strings.Join(*to, ","),
	}

	tmplData.Execute(bufData, dataProperties)

	return bufData
}

func main() {
	kingpin.Parse()
	start := time.Now()
	log.Info("Sending Mail.")
	/*	err := smtp.SendMail(*server, nil, *from, *to, []byte(*data))
		if err != nil {
			log.Fatal(err)
		}*/
	log.Infof("=== Trying %s", *server)
	c, err := smtpclient.Dial(*server)
	if err != nil {
		log.WithError(err).Fatalf("")
	}
	defer c.Close()
	log.Infof("=== Connected to %s", *server)

	log.Infof("-> HELO %s", *helo)
	if err = c.Hello(*helo); err != nil {
		log.WithError(err).Fatalf("")
	}

	log.Infof("-> MAIL FROM:<%s>", *from)
	if err = c.Mail(*from); err != nil {
		log.WithError(err).Fatalf("")
	}

	for _, addr := range *to {
		log.Infof("-> RCPT TO:<%s>", addr)
		if err = c.Rcpt(addr); err != nil {
			log.WithError(err).Errorf("")
		}
	}

	log.Infof("-> DATA")
	w, err := c.Data()
	if err != nil {
		log.WithError(err).Fatalf("")
	}

	if *dataFromFile == nil {
		//_, err = io.Copy(w, strings.NewReader(*data))
		dataBuf := renderTemplate(*data)
		log.Infof("-> %s", dataBuf.String())
		_, err = io.Copy(w, dataBuf)
		if err != nil {
			log.WithError(err).Fatalf("")
		}
		if *extentDataFactor > 0 {
			log.Infof("-> .... extent data with 1kb * %d", *extentDataFactor)
			b := new(bytes.Buffer)
			for i := 0; i < *extentDataFactor; i++ {
				for i := 0; i < 11; i++ {
					b.WriteString("12345678901234567890123456789012345678901234567890123456789001234567890012345678900123456789001234\r\n")
				}
			}
			l := b.Len()
			_, err = io.Copy(w, b)
			if err != nil {
				log.WithError(err).Fatalf("")
			}
			log.Infof("added %dkb", l/1024)
			log.Infof("-> .")
		}
	} else {
		log.Infof("-> .... content from file")
		_, err = io.Copy(w, *dataFromFile)
		if err != nil {
			log.WithError(err).Fatalf("")
		}
		log.Infof("-> .")
	}

	err = w.Close()
	if err != nil {
		log.WithError(err).Fatalf("")
	}
	log.Infof("-> QUIT")
	err = c.Quit()
	if err != nil {
		log.WithError(err).Fatalf("")
	}

	log.Infof("Took %s", time.Since(start))

}

func GenRandomID(length int) string {
	var letters = []rune("bcdfghjklmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ")
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
