package pkg

import (
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/jdkato/prose/v2"
)

// BM25Calculator BM25计算器
// BM25是TF-IDF的改进版本，提供更好的文档长度归一化和词频权重计算
type BM25Calculator struct {
	documents    []string
	vocabulary   map[string]int
	docLengths   []int              // 每个文档的长度（词数）
	avgDocLength float64            // 平均文档长度
	docFreq      map[string]int     // 文档频率
	totalDocs    int                // 总文档数
	k1           float64            // BM25参数k1（通常1.2-2.0）
	b            float64            // BM25参数b（通常0.75）
	bm25Cache    map[string]float64 // BM25分数缓存
	mu           sync.RWMutex
}

// NewBM25Calculator 创建BM25计算器
func NewBM25Calculator(documents []string) *BM25Calculator {
	calc := &BM25Calculator{
		documents:  documents,
		vocabulary: make(map[string]int),
		docLengths: make([]int, len(documents)),
		docFreq:    make(map[string]int),
		totalDocs:  len(documents),
		k1:         1.5,  // 推荐值
		b:          0.75, // 推荐值
		bm25Cache:  make(map[string]float64),
	}
	calc.buildVocabulary()
	return calc
}

// Calculate 计算文本与文档集合的BM25相似度分数
func (calc *BM25Calculator) Calculate(text string) map[string]float64 {
	queryWords := calc.tokenize(text)

	calc.mu.RLock()
	defer calc.mu.RUnlock()

	result := make(map[string]float64)

	// 计算每个文档的BM25分数
	for i, doc := range calc.documents {
		score := calc.calculateBM25Score(queryWords, i)
		result[doc] = score
	}

	return result
}

// CalculateQuerySimilarity 计算查询与单个文档的相似度
func (calc *BM25Calculator) CalculateQuerySimilarity(query, document string) float64 {
	queryWords := calc.tokenize(query)
	docWords := calc.tokenize(document)

	// 计算文档长度
	docLength := len(docWords)

	// 计算平均文档长度（使用现有文档集合的平均值）
	avgLength := calc.avgDocLength
	if avgLength == 0 {
		avgLength = float64(docLength)
	}

	// 计算词频
	docWordFreq := make(map[string]int)
	for _, word := range docWords {
		docWordFreq[word]++
	}

	// 计算BM25分数
	score := 0.0
	for _, word := range queryWords {
		// 计算IDF
		idf := calc.calculateIDF(word)

		// 计算词频
		tf := float64(docWordFreq[word])

		// BM25公式
		if tf > 0 {
			numerator := tf * (calc.k1 + 1)
			denominator := tf + calc.k1*(1-calc.b+calc.b*float64(docLength)/avgLength)
			score += idf * numerator / denominator
		}
	}

	return score
}

// ExtractKeywords 提取关键词（重要性排序）
func (calc *BM25Calculator) ExtractKeywords(text string, topN int) []string {
	words := calc.tokenize(text)

	// 计算每个词的BM25重要性分数
	type wordScore struct {
		word  string
		score float64
	}

	var wordScores []wordScore
	wordFreq := make(map[string]int)

	// 统计词频
	for _, word := range words {
		wordFreq[word]++
	}

	// 计算每个词的分数
	for word, freq := range wordFreq {
		idf := calc.calculateIDF(word)
		tf := float64(freq) / float64(len(words))

		// 使用改进的BM25公式计算词的重要性
		score := idf * tf * math.Log(float64(calc.totalDocs+1))
		wordScores = append(wordScores, wordScore{word, score})
	}

	// 按分数降序排序
	sort.Slice(wordScores, func(i, j int) bool {
		return wordScores[i].score > wordScores[j].score
	})

	// 返回前N个关键词
	var result []string
	for i := 0; i < topN && i < len(wordScores); i++ {
		result = append(result, wordScores[i].word)
	}

	return result
}

// AddDocument 添加新文档（增量更新）
func (calc *BM25Calculator) AddDocument(doc string) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	calc.documents = append(calc.documents, doc)
	words := calc.tokenize(doc)
	docLength := len(words)

	// 更新文档长度数组
	calc.docLengths = append(calc.docLengths, docLength)

	// 更新平均文档长度
	totalLength := 0
	for _, length := range calc.docLengths {
		totalLength += length
	}
	calc.avgDocLength = float64(totalLength) / float64(len(calc.docLengths))

	// 更新词汇表和文档频率
	uniqueWords := make(map[string]bool)
	for _, word := range words {
		calc.vocabulary[word]++
		uniqueWords[word] = true
	}

	for word := range uniqueWords {
		calc.docFreq[word]++
	}

	calc.totalDocs = len(calc.documents)

	// 清除缓存
	calc.bm25Cache = make(map[string]float64)
}

// tokenize 使用prose进行分词
func (calc *BM25Calculator) tokenize(text string) []string {
	doc, err := prose.NewDocument(text)
	if err != nil {
		// prose分词失败，返回空切片
		return []string{}
	}

	var words []string
	for _, tok := range doc.Tokens() {
		word := strings.ToLower(strings.TrimSpace(tok.Text))
		if len(word) > 1 { // 过滤单字符词
			words = append(words, word)
		}
	}
	return words
}

// calculateBM25Score 计算BM25分数
func (calc *BM25Calculator) calculateBM25Score(queryWords []string, docIndex int) float64 {
	docLength := calc.docLengths[docIndex]
	docWords := calc.tokenize(calc.documents[docIndex])

	// 计算文档词频
	docWordFreq := make(map[string]int)
	for _, word := range docWords {
		docWordFreq[word]++
	}

	score := 0.0
	for _, word := range queryWords {
		// 计算IDF
		idf := calc.calculateIDF(word)

		// 计算词频
		tf := float64(docWordFreq[word])

		// BM25公式
		if tf > 0 {
			numerator := tf * (calc.k1 + 1)
			denominator := tf + calc.k1*(1-calc.b+calc.b*float64(docLength)/calc.avgDocLength)
			score += idf * numerator / denominator
		}
	}

	return score
}

// calculateIDF 计算逆文档频率
func (calc *BM25Calculator) calculateIDF(word string) float64 {
	if calc.totalDocs == 0 {
		return 0
	}

	df := calc.docFreq[word]
	if df == 0 {
		df = 1 // 避免除以零
	}

	// 使用平滑的IDF公式
	idf := math.Log(float64(calc.totalDocs-df)+0.5) / (float64(df) + 0.5)
	return math.Max(0, idf) // 确保IDF非负
}

// buildVocabulary 构建词汇表和统计信息
func (calc *BM25Calculator) buildVocabulary() {
	totalLength := 0

	for i, doc := range calc.documents {
		words := calc.tokenize(doc)
		docLength := len(words)
		calc.docLengths[i] = docLength
		totalLength += docLength

		// 统计文档频率
		uniqueWords := make(map[string]bool)
		for _, word := range words {
			calc.vocabulary[word]++
			uniqueWords[word] = true
		}

		for word := range uniqueWords {
			calc.docFreq[word]++
		}
	}

	// 计算平均文档长度
	if len(calc.documents) > 0 {
		calc.avgDocLength = float64(totalLength) / float64(len(calc.documents))
	}

	calc.totalDocs = len(calc.documents)
}

// GetVocabularySize 获取词汇表大小
func (calc *BM25Calculator) GetVocabularySize() int {
	calc.mu.RLock()
	defer calc.mu.RUnlock()
	return len(calc.vocabulary)
}

// GetDocumentCount 获取文档数量
func (calc *BM25Calculator) GetDocumentCount() int {
	calc.mu.RLock()
	defer calc.mu.RUnlock()
	return calc.totalDocs
}

// SetParameters 设置BM25参数
func (calc *BM25Calculator) SetParameters(k1, b float64) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	calc.k1 = k1
	calc.b = b

	// 清除缓存
	calc.bm25Cache = make(map[string]float64)
}
