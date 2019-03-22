package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/unicode"
)

var (
	read    = binary.Read
	le      = binary.LittleEndian
	decoder = unicode.All[3].NewDecoder // UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
)

var cel struct {
	hzr      uint16 // hanzi table offset
	pinyinr  uint32 // pin yin table offset
	wordc    int32  // 词条数量
	ver      uint8
	name     string
	category string
	desc     string
	samples  string
	dict     map[uint16]string
}

type word struct {
	hanzi  string
	pinyin string
	freq   uint64
}

func withSuffix(fn, ext string) string {
	x := filepath.Ext(fn)
	return strings.TrimSuffix(fn, x) + ext
}
func trans(file string) {
	defer xrecover()
	target := withSuffix(file, ".txt")
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0644)
	panice(err)
	defer func() { _ = output.Close() }()

	scel, err := os.Open(file)
	panice(err)
	defer func() { _ = scel.Close() }()

	cel.hzr = 0x2628
	_ = read(scel, le, &cel.pinyinr) // 0x1540

	_ = read(scel, le, &cel.ver)
	if cel.ver == 0x45 { // ?
		cel.hzr = 0x26c4
	}

	_, err = scel.Seek(0x124, 0) // wordc offset
	panice(err)
	_ = read(scel, le, &cel.wordc)

	cel.name = reads(scel, 0x130, 0x200)     // 字库名称
	cel.category = reads(scel, 0x338, 0x200) // 字库类别
	cel.desc = reads(scel, 0x540, 0x800)     // 字库信息
	cel.samples = reads(scel, 0xd40, 0x800)  // 字库示例

	_, err = scel.Seek(int64(cel.pinyinr), 0) // 拼音库
	panice(err)
	cel.dict, _ = pinyindict(scel)

	_, err = scel.Seek(int64(cel.hzr), 0)

	for wordc := cel.wordc; err == nil && wordc > 0; wordc-- {
		var wordsc uint16
		err = read(scel, le, &wordsc)

		pinyin, wordlen := pinyins(scel, cel.dict)

		for i := 0; i < int(wordsc); i++ {
			hz, _ := bstr(scel)
			if len([]rune(hz)) != wordlen { // failed http://pinyin.sogou.com/dict/detail/index/4
				break
			}
			var wl uint16
			_ = read(scel, le, &wl) // 0x0a

			var weight uint64
			_ = read(scel, le, &weight)

			unknown := make([]byte, 2)
			_ = read(scel, le, &unknown)
			if config.withPinYin {
				_, _ = fmt.Fprintf(output, "%s\t%s\n", hz, pinyin)
			} else {
				_, _ = fmt.Fprintln(output, hz)
			}
		}
	}
}
func xrecover() {
	if err := recover(); err != nil {
		log.Println(err)
	}
}

// 搜狗细胞词库
func main() {
	files := []string{config.input}
	f, err := os.Stat(config.input)
	if err != nil {
		log.Fatal(err)
	}
	if f.IsDir() {
		files, err = filepath.Glob(filepath.Join(config.input, "*.scel"))
	}
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		log.Println(file)
		trans(file)
	}
}
func pinyins(scel io.Reader, dict map[uint16]string) (string, int) {
	var pyc uint16
	_ = read(scel, le, &pyc)
	var pys = make([]uint16, pyc/2)
	_ = read(scel, le, pys)
	return pinyin(dict, pys), int(pyc / 2)
}
func pinyindict(scel io.Reader) (map[uint16]string, error) {
	pydict := map[uint16]string{}
	var cnt int32
	err := read(scel, le, &cnt)
	panice(err)

	for ; cnt > 0; cnt-- { // or py != "zuo"
		var idx uint16
		_ = read(scel, le, &idx)

		py, _ := bstr(scel)
		pydict[idx] = py
	}
	return pydict, err
}
func pinyin(dict map[uint16]string, indexes []uint16) string {
	var pys = make([]string, len(indexes))
	for i, idx := range indexes {
		pys[i] = dict[idx]
	}
	return strings.Join(pys, "'")
}

func bstr(f io.Reader) (ret string, err error) {
	var l uint16
	_ = read(f, le, &l)
	var data = make([]byte, l)
	err = read(f, le, data)
	data, _ = decoder().Bytes(data)
	ret = string(data)

	return
}

func reads(f io.ReadSeeker, offset int64, len int) string {
	_, _ = f.Seek(offset, 0)
	data := make([]byte, len)
	_ = read(f, le, data)
	data, _ = decoder().Bytes(data)
	return trim(string(data))
}

func trim(str string) string {
	return strings.TrimRightFunc(str, func(r rune) bool {
		return r == 0
	})
}

func panice(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {
	flag.StringVar(&config.input, "input", ".", "")
	flag.BoolVar(&config.withPinYin, "with-pinyin", false, "")

	flag.Parse()
}

var config struct {
	input      string
	withPinYin bool
}
