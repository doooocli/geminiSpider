package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/generative-ai-go/genai"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	promptText = `Tulis artikel 1500 kata yang dioptimalkan untuk seo tentang \"game\".\n                Gunakan tag <h1>untuk judul artikel keseluruhan dan tag <h2> untuk judul bagian.\n                Selalu gunakan nada emosi dan prioritaskan kejelasan daripada struktur kalimat yang rumit.\n                Gunakan personifikasi, metafora, pertanyaan retoris, pertanyaan retoris, dan kontras dalam setiap kalimat agar tidak berlebihan.\n                Lebih banyak menggunakan pertanyaan retoris dan perbandingan.\n                Tulis judul pendek yang layak untuk diklik.\n                Setiap kalimat hendaknya menggunakan pola kalimat khusus seperti inversi dan penghilangan untuk mendiversifikasi bentuk penulisan.\n                Tulis perkenalan kepada penulis dan hasilkan satu penulis asli dan cantumkan tautan profil linkedin asli.\n                Langkah 1 - hasilkan 6 pengetahuan keahlian atau tip tentang \"game\".\n                Langkah 1 - hasilkan 6 pengetahuan keahlian atau tip tentang \"game\".\n                Langkah 2 - ambil kata kunci pertama dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, daftar dan tabel, solusi detail dengan pendekatan langkah demi langkah.\n                Langkah 3 - ambil kata kunci ke-2 dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, solusi terperinci, dan buat satu tabel yang berguna.\n                Langkah 4 - ambil kata kunci ke-3 dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, solusi detail.\n                Langkah 5 - ambil kata kunci ke-4 dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, solusi detail, dan buat satu daftar yang berguna.\n                Langkah 6 - ambil kata kunci ke-5 dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, daftar dan tabel, solusi terperinci dengan pendekatan langkah demi langkah.\n                Langkah 7 - ambil kata kunci ke-6 dari daftar dari Langkah 1 dan tulis artikel 10 paragraf menggunakan format penurunan harga, daftar dan tabel, solusi detail, dan hasilkan satu tabel yang berguna.\n                Hasilkan 6 manfaat bagi pengguna.\n                Hasilkan dua tabel berguna untuk artikel di atas dan integrasikan secara alami.\n                Menghasilkan sejumlah besar angka yang diterbitkan oleh banyak organisasi berwenang dan menyediakan sumber serta informasi pengecekan fakta.\n                Hasilkan 2 organisasi otoritatif dan informasi pengecekan fakta untuk membantu orang.\n                Hasilkan 5 tips berguna untuk berbagi.\n                Hasilkan 5 faq berguna diikuti dengan skema FAQPage.`

	promptText2 = "Explain how AI works"

	promptjson = `{
    "contents": 
    [
        {
            "parts":[
                {
                    "text": "#PROMPT_TEXT#"
            }
            ]
        }
    ]
}`
)

type genRet struct {
	key      string
	proxyUrl string
	prompt   string
	content  string
}

type genErr struct {
	key     string
	message string
}

type GeminiSpider struct {
	ctx             context.Context
	cancel          func()
	scheduler       *Scheduler          // 并发调度
	keyPool         []string            // key池
	dirtyKeys       map[string]struct{} // 脏key
	keyProxy        map[string]string   // 代理池
	genaiClientPool *GenaiClientPool    // gemini客户端
	retCh           chan *genRet        // 生成内容异步入库
	errCh           chan *genErr        // 异步处理错误
}

func NewGeminiSpider(ctx context.Context) *GeminiSpider {

	ctx, cancel := context.WithCancel(ctx)
	return &GeminiSpider{
		ctx:       ctx,
		cancel:    cancel,
		scheduler: NewScheduler(ctx, viper.GetInt("app.workers")),
		keyPool:   loadKeys(),
		dirtyKeys: make(map[string]struct{}),
		genaiClientPool: &GenaiClientPool{
			httpProxyPool: viper.GetStringSlice("app.httpProxyPool"),
		},
		retCh: make(chan *genRet, 10000),
		errCh: make(chan *genErr, 10000),
	}
}

type generateContentResponse struct {
	Candidates []struct {
		Content *struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	PromptFeedback struct {
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"promptFeedback"`
}

// 不使用gemini官方sdk
func (g *GeminiSpider) handler(ctx context.Context, key string) error {
	proxyUrl := g.genaiClientPool.getProxyURL(key)
	client := NewHttpClient(key, proxyUrl)
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent?key=%s", key),
		bytes.NewBuffer([]byte(strings.Replace(promptjson, "#PROMPT_TEXT#", promptText, -1))),
	)
	if err != nil {
		log.Printf("http.NewRequest err %v\n", err)
		return err
	}
	req.Close = true
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-Type", "application/json")
	log.Printf("gen start %s - %s \n", key, proxyUrl)
	response, err := client.Do(req)
	if err != nil {
		log.Printf("client.Do err %v\n", err)
		return nil
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 {
		log.Printf("o.ReadAll err %d %+v %s \n", response.StatusCode, err, string(data))
		if strings.Contains(err.Error(), "EOF") { // 代理端异常导致
			return nil
		}
		g.errCh <- &genErr{key: key, message: err.Error()}
		return err
	}
	repText := &generateContentResponse{}
	err = json.Unmarshal(data, repText)
	if err != nil {
		log.Printf("json decode err %d %v %s\n", response.StatusCode, err, string(data))
		return nil
	}
	//log.Printf("repText = %s\n", string(data))
	g.printResponse(key, proxyUrl, promptText, repText)
	return err
}

func (g *GeminiSpider) printResponse(key, proxyUrl, prompt string, resp *generateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				g.retCh <- &genRet{
					key:      key,
					proxyUrl: proxyUrl,
					prompt:   prompt,
					content:  fmt.Sprintf("%v", part),
				}
			}
		}
	}
	log.Println("gen done " + key)
	log.Println("---")
}

// 使用官方sdk
func (g *GeminiSpider) handlerWithSdk(ctx context.Context, key string) error {
	proxyUrl := g.genaiClientPool.getProxyURL(key)
	client, err := NewGenClient(ctx, key, proxyUrl)
	if err != nil {
		return err
	}

	if err != nil {
		log.Printf("get NewGenClient err %v\n", err)
		return err
	}
	defer client.Close()
	log.Printf("gen start %s - %s \n", key, proxyUrl)

	model := client.GenerativeModel("gemini-pro")
	repText, err := model.GenerateContent(ctx, genai.Text(promptText))
	if err != nil {
		log.Printf("GenerateContent err %s - %v", key, err)
		if strings.Contains(err.Error(), "EOF") { // 代理端异常导致
			return nil
		}
		g.errCh <- &genErr{key: key, message: err.Error()}
		return err
	}
	g.printSdkResponse(key, proxyUrl, promptText, repText)
	return nil
}

func (g *GeminiSpider) printSdkResponse(key, proxyUrl, prompt string, resp *genai.GenerateContentResponse) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				g.retCh <- &genRet{
					key:      key,
					proxyUrl: proxyUrl,
					prompt:   prompt,
					content:  fmt.Sprintf("%v", part),
				}
			}
		}
	}
	log.Println("gen done " + key)
	log.Println("---")
}

func (g *GeminiSpider) retQueue() {
	for {
		select {
		case ret := <-g.retCh:
			_, err := db.Exec("INSERT INTO `gen` (`key`, `proxy`, `prompt`, `content`) VALUES (?,?,?,?)", ret.key, ret.proxyUrl, ret.prompt, ret.content)
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func (g *GeminiSpider) errQueue() {
	for {
		select {
		case errRet := <-g.errCh:

			// 这是被封了
			//"message": "Quota exceeded for quota metric 'Generate Content API requests per minute' and limit 'GenerateContent request limit per minute for a region' of service 'generativelanguage.googleapis.com' for consumer 'project_num
			if strings.Contains(errRet.message, "Quota exceeded for quota metric") {
				_, err := db.Exec("update `keys` set status = 0, `error` = ? where `key` = ?", errRet.message, errRet.key)
				if err != nil {
					log.Println(err)
				}
				break
			}

			// 其他错误 一律休眠一分钟
			_, err := db.Exec("update `keys` set `active_ts` = ?, `error` = ? where `key` = ?", time.Now().Unix()+60, errRet.message, errRet.key)
			if err != nil {
				log.Println(err)
				break
			}
		}
	}
}

func loadKeys() []string {
	// 从数据库加载key
	ts := time.Now().Unix()
	rows, err := db.Query("select `key` from `keys` where `status` = 1 and `active_ts` <= ?", ts)
	if err != nil {
		log.Printf("loadKeys err %d %v\n", ts, err)
		return []string{}
	}

	var keyPool []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err == nil {
			keyPool = append(keyPool, key)
		}
	}
	log.Printf("loadKeys = %d %v\n", ts, keyPool)
	return keyPool
}

func (g *GeminiSpider) loopKey() {
	for {
		for _, key := range g.keyPool {
			select {
			case <-g.ctx.Done():
				return
			default:
				if _, ok := g.dirtyKeys[key]; ok {
					continue
				}
				g.scheduler.dispatch(func(ctx context.Context) {

					// 直接发送post请求
					//if err := g.handler(ctx, key); err != nil {
					//	log.Println("dirty key " + key)
					//	g.dirtyKeys[key] = struct{}{}
					//}

					// 使用官方的sdk
					if err := g.handlerWithSdk(ctx, key); err != nil {
						log.Println("dirty key " + key)
						g.dirtyKeys[key] = struct{}{}
					}
				})
			}
		}
		// 是否等带执行完再循环key，如果只有单个key，会退化为阻塞模式
		g.scheduler.Wait()

		// 重新加载key
		if len(g.dirtyKeys) == len(g.keyPool) {
			g.dirtyKeys = make(map[string]struct{}) // 清空
			g.keyPool = loadKeys()
			if len(g.keyPool) == 0 {
				// 数据库没key，每十秒查询一次
				<-time.After(10 * time.Second)
			}
		}
	}
}

func (g *GeminiSpider) Start() {
	defer close(g.retCh)
	defer close(g.errCh)
	go g.retQueue() // 处理生成结果
	go g.errQueue() // 处理错误
	g.loopKey()
}

func (g *GeminiSpider) testIP() {
	for _, key := range g.keyPool {
		proxyURL := g.genaiClientPool.getProxyURL(key)
		httpClient := NewHttpClient(key, proxyURL)
		if resp, err := httpClient.Get("https://ipinfo.io/"); err != nil {
			log.Printf("ip check = %s %s %v \n", proxyURL, key, err)
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			log.Printf("ip check = %s %s %s \n", proxyURL, key, string(body))
		}
	}
}

var db *sql.DB

func main() {
	db = initDB()
	ctx := context.Background()
	spider := NewGeminiSpider(ctx)
	if viper.GetBool("app.checkIP") {
		spider.testIP()
	}
	spider.Start()
}
