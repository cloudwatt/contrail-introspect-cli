package collection

import "fmt"
import "os"
import "net/http"
import "io/ioutil"
import "log"
import "strings"
import "strconv"

import "github.com/jbowtie/gokogiri"
import "github.com/jbowtie/gokogiri/xml"

type Remote struct {
	VrouterUrl string
	Port       int
	Table      string
}

type Webui struct {
	VrouterUrl string
	Port       int
	Path       string
}

type File struct {
	Path string
}

type Sourcer interface {
	Load(descCol DescCollection) (Collection, error)
}

func LoadCollection(descCol DescCollection, args []string) (Collection, error) {
	return descCol.PageBuilder(args).Load(descCol)
}

func Load(url string, fromFile bool) *xml.XmlDocument {
	var data []byte
	if fromFile {
		file, _ := os.Open(url)
		data, _ = ioutil.ReadAll(file)
	} else {
		resp, err := http.Get(url)
		if err != nil {
			log.Fatal(err)
		}
		data, _ = ioutil.ReadAll(resp.Body)
	}
	doc, _ := gokogiri.ParseXml(data)
	return (doc)
}

func dataToXml(data []byte) xml.Node {
	doc, _ := gokogiri.ParseXml(data)
	ss, _ := doc.Search("/")
	if len(ss) < 1 {
		log.Fatal("Failed to parse XML")
	}
	return ss[0]
}

func xmlToCollection(node xml.Node, descCol DescCollection, url string) Collection {
	col := Collection{rootNode: node, descCol: descCol, Url: url}
	col.Init()
	return col
}

// Parse data to XML
func fromDataToCollection(data []byte, descCol DescCollection, url string) Collection {
	return xmlToCollection(dataToXml(data), descCol, url)
}

// Split the url with format host:port into host and port. If port is
// not specified, the defaultPort value is used.
func splitUrl(url string, defaultPort int) (host string, port int) {
	elts := strings.Split(url, ":")
	host = elts[0]
	port = defaultPort
	if len(elts) == 2 {
		t, e := strconv.Atoi(elts[1])
		if e != nil {
			log.Printf("Port %s in url %s is not a valid port number. Default port %d is used instead.", elts[1], url, defaultPort)
		} else {
			port = t
		}
	}
	return host, port
}

func (file File) Load(descCol DescCollection) (Collection, error) {
	f, _ := os.Open(file.Path)
	data, _ := ioutil.ReadAll(f)
	return fromDataToCollection(data, descCol, "file://"+file.Path), nil
}

func (page Remote) Load(descCol DescCollection) (Collection, error) {
	host, port := splitUrl(page.VrouterUrl, page.Port)
	url := fmt.Sprintf("http://%s:%d/Snh_PageReq?x=begin:-1,end:-1,table:%s,", host, port, page.Table)
	resp, err := http.Get(url)
	if err != nil {
		return Collection{}, err
	}
	data, _ := ioutil.ReadAll(resp.Body)
	return fromDataToCollection(data, descCol, url), nil
}

func (page Webui) Load(descCol DescCollection) (Collection, error) {
	host, port := splitUrl(page.VrouterUrl, page.Port)
	url := fmt.Sprintf("http://%s:%d/%s", host, port, page.Path)
	var data []byte
	var node xml.Node

	resp, err := http.Get(url)
	if err != nil {
		return Collection{}, err
	}
	data, _ = ioutil.ReadAll(resp.Body)
	node = dataToXml(data)

	// Handle controller pagination
	//
	// Since it is not possible to query controller introspect
	// without pagination, we query it several time the controller
	// to rebuild a xml containing all nodes.
	currentNode := node
	for {
		if r, _ := currentNode.Search("/*/next_batch"); len(r) > 0 {
			currentUrl := fmt.Sprintf("http://%s:%d/Snh_%s?x=%s", page.VrouterUrl, page.Port, r[0].Attribute("link"), r[0].Content())
			resp, err := http.Get(currentUrl)
			if err != nil {
				log.Fatal(err)
			}
			data, _ = ioutil.ReadAll(resp.Body)
			currentNode = dataToXml(data)
			newLists, _ := currentNode.Search("/*/*/list")

			for _, l := range newLists {
				lists, _ := node.Search("/*/*/list")
				lists[len(lists)-1].InsertAfter(l)
			}
		} else {
			break
		}
	}

	return xmlToCollection(node, descCol, url), nil
}
