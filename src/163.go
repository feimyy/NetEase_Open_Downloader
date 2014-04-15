package main

import (
	"bytes"
	"fmt"
	iconv "github.com/feimyy/iconv"
	"io"
	"io/ioutil"
	// "log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type ResourceInfo struct {
	Id      string
	DownUrL string
	Suffix  string // don't contain the dot(.)
	Name    string
}

func checkError(err error) {
	if err == io.EOF {
		return
	} else if err != nil {
		fmt.Println("Occured a error  :", err.Error())
		os.Exit(0)
	}

}

func getSourceCode(Url string) (sourceCode string) {

	rep, err := http.Get(Url)
	checkError(err)

	b, err := ioutil.ReadAll(rep.Body)
	checkError(err)

	sourceCode = fmt.Sprintf("%s", b)
	defer rep.Body.Close()

	return
}
func getPage() string {

	return "http://v.163.com/special/financialmarkets/"
}

func getResourceDownloadList(PageSourceCode string) []ResourceInfo {

	//通过正则粗略匹配所有视频下载Url
	down_exp := "<a\\sclass=\"downbtn\"\\shref='.*?</a>"
	list := getDownloadList(down_exp, PageSourceCode)
	//fmt.Printf("list :%v\n\n", list)

	//根据粗略匹配到Url中的获取精确Url
	filted_list := filterDownloadList(list)
	//fmt.Printf("%v\n\n", filted_list)

	//获取页面的字符集
	charser_exp := "<meta http-equiv=[^>]*?\">"
	charset := getPageCharset(charser_exp, PageSourceCode)
	fmt.Printf("charset :%s\n", charset)

	//获取Url视频资源对应的名称
	for _, v := range filted_list {
		name_exp := fmt.Sprintf("http://v\\.163\\.com.*?%s[^<]*?</a>", v.Id)
		name := getResourceNameById(name_exp, PageSourceCode)

		converter, err := iconv.Open("utf-8", charset)
		if err != nil {
			fmt.Printf("iconv.Open() error :%s\n", err)
			panic(err)
		}

		name_reader := bytes.NewReader(name)
		converted_reader := iconv.NewReader(converter, name_reader, len(name))

		converted_name_bytes, _ := ioutil.ReadAll(converted_reader)
		buffer := bytes.NewBuffer(converted_name_bytes)
		v.Name = buffer.String() //保存转化后的名称
		fmt.Printf("Id :%s ,Name :%s\n\n", v.Id, v.Name)

	}

	return filted_list
}

//通过正则获取包含了视频下载链接的内容段
func getDownloadList(exp string, PageSourceCode string) []string {
	reg, err := regexp.Compile(exp)
	checkError(err)
	list := reg.FindAllString(PageSourceCode, -1)
	return list
}

/*
   从正则表达式粗略匹配的包含了下载的内容段中，
   精确匹配出视频Url,后缀,Id
*/
func filterDownloadList(DownloadList []string) []ResourceInfo {
	down_exp := "href='[^']*?'"
	id_exp := "id='[^']*?'"
	down_reg, _ := regexp.Compile(down_exp)
	id_reg, _ := regexp.Compile(id_exp)

	ResourceList := make([]ResourceInfo, len(DownloadList))
	for i, v := range DownloadList {
		down_content := down_reg.FindString(v)
		id_content := id_reg.FindString(v)

		//fmt.Printf("down_content :%s ,id_content:%s\n", down_content, id_content)
		down_content_splited := strings.Split(down_content, "'")
		id_content_splited := strings.Split(id_content, "'")

		//fmt.Printf("len :%d ,down_content_splited :%v\n", len(down_content_splited), down_content_splited)
		//fmt.Printf("len : %d ,id_content_splited :%v\n", len(id_content_splited), id_content_splited)

		ResourceList[i].DownUrL = down_content_splited[len(down_content_splited)-2]
		ResourceList[i].Id = id_content_splited[len(id_content_splited)-2]

		//从下载Url中提取后缀名
		DownUrl_splited := strings.Split(ResourceList[i].DownUrL, ".")
		ResourceList[i].Suffix = DownUrl_splited[len(DownUrl_splited)-1]
	}

	return ResourceList
}

//获取原页面的字符编码
func getPageCharset(exp string, PageSourceCode string) string {

	charset := "gb2312" // default charset
	reg, _ := regexp.Compile(exp)
	meta := reg.FindString(PageSourceCode)
	if !strings.Contains(meta, "charset=") {
		return charset
	}

	contents := strings.Split(meta, "charset=")
	charset_content := contents[1]

	start := 0
	end := len(charset_content)
	for i := 0; i < len(charset_content); i++ {
		if charset_content[i] == '"' {
			end = i
			break
		}
	}

	charset = charset_content[start:end]
	return charset
}

/*
   通过视频下载Url的Id获取匹配视频的名称

   注意:返回的是原页面字符集编码的名称的[]byte数据
*/
func getResourceNameById(exp string, PageSourceCode string) []byte {

	reg, _ := regexp.Compile(exp)
	content := reg.Find([]byte(PageSourceCode))
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '>' {
			start = i
			break
		}
	}

	end := len(content)
	for i := (len(content) - 1); i > 0; i-- {
		if content[i] == '<' {
			end = i
			break
		}
	}

	name := content[start:end]
	return name
}

func main() {

	Url := getPage()
	sourceCode := getSourceCode(Url)
	getResourceDownloadList(sourceCode)

}
