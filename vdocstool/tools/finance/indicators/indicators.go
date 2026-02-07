// Package indicators 技术指标计算引擎
package indicators

import (
	"fmt"
	"math"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Calculator 技术指标计算器
type Calculator struct{}

// NewCalculator 创建计算器
func NewCalculator() *Calculator {
	return &Calculator{}
}

// MA 简单移动平均线
func (c *Calculator) MA(klines []*types.KLine, period int) []float64 {
	if len(klines) < period {
		return nil
	}

	result := make([]float64, len(klines))
	var sum float64

	for i := 0; i < len(klines); i++ {
		sum += klines[i].Close
		if i >= period {
			sum -= klines[i-period].Close
		}
		if i >= period-1 {
			result[i] = sum / float64(period)
		}
	}

	return result
}

// EMA 指数移动平均线
func (c *Calculator) EMA(klines []*types.KLine, period int) []float64 {
	if len(klines) < period {
		return nil
	}

	result := make([]float64, len(klines))
	multiplier := 2.0 / (float64(period) + 1)

	// 第一个EMA使用SMA
	var sum float64
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	result[period-1] = sum / float64(period)

	// 计算后续EMA
	for i := period; i < len(klines); i++ {
		result[i] = (klines[i].Close-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// MACD 指数平滑异同移动平均线
type MACDResult struct {
	DIF       []float64 // 快线
	DEA       []float64 // 慢线
	MACD      []float64 // 柱状图
	Signal    string    // 信号: buy/sell/neutral
	CrossType string    // 交叉类型: golden_cross/death_cross/none
}

func (c *Calculator) MACD(klines []*types.KLine, fastPeriod, slowPeriod, signalPeriod int) *MACDResult {
	if len(klines) < slowPeriod {
		return nil
	}

	emaFast := c.EMA(klines, fastPeriod)
	emaSlow := c.EMA(klines, slowPeriod)

	// 计算DIF
	dif := make([]float64, len(klines))
	for i := slowPeriod - 1; i < len(klines); i++ {
		dif[i] = emaFast[i] - emaSlow[i]
	}

	// 计算DEA (DIF的EMA)
	dea := make([]float64, len(klines))
	multiplier := 2.0 / (float64(signalPeriod) + 1)
	
	startIdx := slowPeriod - 1 + signalPeriod - 1
	if startIdx >= len(klines) {
		return nil
	}

	// DEA初始值
	var sum float64
	for i := slowPeriod - 1; i < slowPeriod-1+signalPeriod; i++ {
		sum += dif[i]
	}
	dea[startIdx] = sum / float64(signalPeriod)

	// 计算后续DEA
	for i := startIdx + 1; i < len(klines); i++ {
		dea[i] = (dif[i]-dea[i-1])*multiplier + dea[i-1]
	}

	// 计算MACD柱状图
	macd := make([]float64, len(klines))
	for i := startIdx; i < len(klines); i++ {
		macd[i] = (dif[i] - dea[i]) * 2
	}

	result := &MACDResult{
		DIF:  dif,
		DEA:  dea,
		MACD: macd,
	}

	// 判断信号
	if len(klines) >= 2 {
		lastIdx := len(klines) - 1
		prevIdx := len(klines) - 2

		if dif[lastIdx] > dea[lastIdx] && dif[prevIdx] <= dea[prevIdx] {
			result.Signal = "buy"
			result.CrossType = "golden_cross"
		} else if dif[lastIdx] < dea[lastIdx] && dif[prevIdx] >= dea[prevIdx] {
			result.Signal = "sell"
			result.CrossType = "death_cross"
		} else if dif[lastIdx] > dea[lastIdx] {
			result.Signal = "buy"
			result.CrossType = "none"
		} else {
			result.Signal = "sell"
			result.CrossType = "none"
		}
	}

	return result
}

// RSI 相对强弱指标
type RSIResult struct {
	Values []float64
	Signal string // oversold(超卖)/overbought(超买)/neutral
}

func (c *Calculator) RSI(klines []*types.KLine, period int) *RSIResult {
	if len(klines) < period+1 {
		return nil
	}

	gains := make([]float64, len(klines))
	losses := make([]float64, len(klines))

	// 计算涨跌
	for i := 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains[i] = change
		} else {
			losses[i] = -change
		}
	}

	// 计算RSI
	result := make([]float64, len(klines))
	
	// 第一个RSI
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		result[period] = 100
	} else {
		result[period] = 100 - 100/(1+avgGain/avgLoss)
	}

	// 后续RSI使用平滑算法
	for i := period + 1; i < len(klines); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)

		if avgLoss == 0 {
			result[i] = 100
		} else {
			result[i] = 100 - 100/(1+avgGain/avgLoss)
		}
	}

	rsiResult := &RSIResult{Values: result}

	// 判断信号
	lastRSI := result[len(result)-1]
	if lastRSI < 30 {
		rsiResult.Signal = "oversold"
	} else if lastRSI > 70 {
		rsiResult.Signal = "overbought"
	} else {
		rsiResult.Signal = "neutral"
	}

	return rsiResult
}

// KDJ 随机指标
type KDJResult struct {
	K      []float64
	D      []float64
	J      []float64
	Signal string // buy/sell/neutral
}

func (c *Calculator) KDJ(klines []*types.KLine, period, kPeriod, dPeriod int) *KDJResult {
	if len(klines) < period {
		return nil
	}

	rsv := make([]float64, len(klines))
	k := make([]float64, len(klines))
	d := make([]float64, len(klines))
	j := make([]float64, len(klines))

	for i := period - 1; i < len(klines); i++ {
		// 计算周期内最高价和最低价
		highest := klines[i].High
		lowest := klines[i].Low
		for j := i - period + 1; j < i; j++ {
			if klines[j].High > highest {
				highest = klines[j].High
			}
			if klines[j].Low < lowest {
				lowest = klines[j].Low
			}
		}

		// 计算RSV
		if highest == lowest {
			rsv[i] = 50
		} else {
			rsv[i] = (klines[i].Close - lowest) / (highest - lowest) * 100
		}

		// 计算K值 (RSV的EMA)
		if i == period-1 {
			k[i] = 50
		} else {
			k[i] = (2*k[i-1] + rsv[i]) / 3
		}

		// 计算D值 (K的EMA)
		if i == period-1 {
			d[i] = 50
		} else {
			d[i] = (2*d[i-1] + k[i]) / 3
		}

		// 计算J值
		j[i] = 3*k[i] - 2*d[i]
	}

	result := &KDJResult{K: k, D: d, J: j}

	// 判断信号
	if len(klines) >= 2 {
		lastIdx := len(klines) - 1
		prevIdx := len(klines) - 2

		if k[lastIdx] > d[lastIdx] && k[prevIdx] <= d[prevIdx] {
			result.Signal = "buy"
		} else if k[lastIdx] < d[lastIdx] && k[prevIdx] >= d[prevIdx] {
			result.Signal = "sell"
		} else {
			result.Signal = "neutral"
		}
	}

	return result
}

// BOLL 布林带
type BOLLResult struct {
	Upper  []float64 // 上轨
	Middle []float64 // 中轨
	Lower  []float64 // 下轨
	Width  []float64 // 带宽
	Signal string    // buy/sell/neutral
}

func (c *Calculator) BOLL(klines []*types.KLine, period int, multiplier float64) *BOLLResult {
	if len(klines) < period {
		return nil
	}

	middle := c.MA(klines, period)
	upper := make([]float64, len(klines))
	lower := make([]float64, len(klines))
	width := make([]float64, len(klines))

	for i := period - 1; i < len(klines); i++ {
		// 计算标准差
		var sum float64
		for j := i - period + 1; j <= i; j++ {
			diff := klines[j].Close - middle[i]
			sum += diff * diff
		}
		std := math.Sqrt(sum / float64(period))

		upper[i] = middle[i] + multiplier*std
		lower[i] = middle[i] - multiplier*std
		if middle[i] != 0 {
			width[i] = (upper[i] - lower[i]) / middle[i] * 100
		}
	}

	result := &BOLLResult{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
		Width:  width,
	}

	// 判断信号
	if len(klines) >= 1 {
		lastIdx := len(klines) - 1
		price := klines[lastIdx].Close

		if price < lower[lastIdx] {
			result.Signal = "buy" // 跌破下轨，超卖
		} else if price > upper[lastIdx] {
			result.Signal = "sell" // 突破上轨，超买
		} else {
			result.Signal = "neutral"
		}
	}

	return result
}

// WR 威廉指标
type WRResult struct {
	Values []float64
	Signal string
}

func (c *Calculator) WR(klines []*types.KLine, period int) *WRResult {
	if len(klines) < period {
		return nil
	}

	wr := make([]float64, len(klines))

	for i := period - 1; i < len(klines); i++ {
		highest := klines[i].High
		lowest := klines[i].Low
		for j := i - period + 1; j < i; j++ {
			if klines[j].High > highest {
				highest = klines[j].High
			}
			if klines[j].Low < lowest {
				lowest = klines[j].Low
			}
		}

		if highest == lowest {
			wr[i] = 50
		} else {
			wr[i] = (highest - klines[i].Close) / (highest - lowest) * 100
		}
	}

	result := &WRResult{Values: wr}

	// 判断信号
	lastWR := wr[len(wr)-1]
	if lastWR > 80 {
		result.Signal = "oversold"
	} else if lastWR < 20 {
		result.Signal = "overbought"
	} else {
		result.Signal = "neutral"
	}

	return result
}

// CCI 商品路径指标
type CCIResult struct {
	Values []float64
	Signal string
}

func (c *Calculator) CCI(klines []*types.KLine, period int) *CCIResult {
	if len(klines) < period {
		return nil
	}

	cci := make([]float64, len(klines))

	for i := period - 1; i < len(klines); i++ {
		// 计算典型价格 (TP)
		var tpSum float64
		for j := i - period + 1; j <= i; j++ {
			tp := (klines[j].High + klines[j].Low + klines[j].Close) / 3
			tpSum += tp
		}
		tpMA := tpSum / float64(period)

		// 计算平均绝对偏差 (MD)
		var mdSum float64
		for j := i - period + 1; j <= i; j++ {
			tp := (klines[j].High + klines[j].Low + klines[j].Close) / 3
			mdSum += math.Abs(tp - tpMA)
		}
		md := mdSum / float64(period)

		// 计算CCI
		tp := (klines[i].High + klines[i].Low + klines[i].Close) / 3
		if md != 0 {
			cci[i] = (tp - tpMA) / (0.015 * md)
		}
	}

	result := &CCIResult{Values: cci}

	// 判断信号
	lastCCI := cci[len(cci)-1]
	if lastCCI < -100 {
		result.Signal = "oversold"
	} else if lastCCI > 100 {
		result.Signal = "overbought"
	} else {
		result.Signal = "neutral"
	}

	return result
}

// ATR 真实波动幅度均值
func (c *Calculator) ATR(klines []*types.KLine, period int) []float64 {
	if len(klines) < period+1 {
		return nil
	}

	tr := make([]float64, len(klines))
	atr := make([]float64, len(klines))

	// 计算真实波动幅度
	for i := 1; i < len(klines); i++ {
		hl := klines[i].High - klines[i].Low
		hpc := math.Abs(klines[i].High - klines[i-1].Close)
		lpc := math.Abs(klines[i].Low - klines[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hpc, lpc))
	}

	// 计算ATR
	var sum float64
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	atr[period] = sum / float64(period)

	for i := period + 1; i < len(klines); i++ {
		atr[i] = (atr[i-1]*float64(period-1) + tr[i]) / float64(period)
	}

	return atr
}

// OBV 能量潮
func (c *Calculator) OBV(klines []*types.KLine) []float64 {
	if len(klines) < 2 {
		return nil
	}

	obv := make([]float64, len(klines))
	obv[0] = float64(klines[0].Volume)

	for i := 1; i < len(klines); i++ {
		if klines[i].Close > klines[i-1].Close {
			obv[i] = obv[i-1] + float64(klines[i].Volume)
		} else if klines[i].Close < klines[i-1].Close {
			obv[i] = obv[i-1] - float64(klines[i].Volume)
		} else {
			obv[i] = obv[i-1]
		}
	}

	return obv
}

// VOL 成交量均线
type VOLResult struct {
	Volume []int64
	MA5    []float64
	MA10   []float64
	MA20   []float64
}

func (c *Calculator) VOL(klines []*types.KLine) *VOLResult {
	if len(klines) < 5 {
		return nil
	}

	result := &VOLResult{
		Volume: make([]int64, len(klines)),
		MA5:    make([]float64, len(klines)),
		MA10:   make([]float64, len(klines)),
		MA20:   make([]float64, len(klines)),
	}

	var sum5, sum10, sum20 float64

	for i := 0; i < len(klines); i++ {
		result.Volume[i] = klines[i].Volume
		vol := float64(klines[i].Volume)

		sum5 += vol
		sum10 += vol
		sum20 += vol

		if i >= 5 {
			sum5 -= float64(klines[i-5].Volume)
		}
		if i >= 10 {
			sum10 -= float64(klines[i-10].Volume)
		}
		if i >= 20 {
			sum20 -= float64(klines[i-20].Volume)
		}

		if i >= 4 {
			result.MA5[i] = sum5 / 5
		}
		if i >= 9 {
			result.MA10[i] = sum10 / 10
		}
		if i >= 19 {
			result.MA20[i] = sum20 / 20
		}
	}

	return result
}

// Analyze 综合分析
func (c *Calculator) Analyze(klines []*types.KLine) *types.AnalysisReport {
	if len(klines) < 30 {
		return nil
	}

	report := &types.AnalysisReport{
		Indicators: make([]*types.TechnicalIndicator, 0),
	}

	if len(klines) > 0 {
		report.Symbol = klines[0].Symbol
	}

	lastIdx := len(klines) - 1

	// MA
	ma5 := c.MA(klines, 5)
	ma10 := c.MA(klines, 10)
	ma20 := c.MA(klines, 20)
	
	maSignal := "neutral"
	if ma5[lastIdx] > ma10[lastIdx] && ma10[lastIdx] > ma20[lastIdx] {
		maSignal = "buy"
	} else if ma5[lastIdx] < ma10[lastIdx] && ma10[lastIdx] < ma20[lastIdx] {
		maSignal = "sell"
	}
	
	report.Indicators = append(report.Indicators, &types.TechnicalIndicator{
		Name: "MA",
		Values: map[string]float64{
			"MA5":  ma5[lastIdx],
			"MA10": ma10[lastIdx],
			"MA20": ma20[lastIdx],
		},
		Signal: maSignal,
	})

	// MACD
	macd := c.MACD(klines, 12, 26, 9)
	if macd != nil {
		report.Indicators = append(report.Indicators, &types.TechnicalIndicator{
			Name: "MACD",
			Values: map[string]float64{
				"DIF":  macd.DIF[lastIdx],
				"DEA":  macd.DEA[lastIdx],
				"MACD": macd.MACD[lastIdx],
			},
			Signal: macd.Signal,
		})
	}

	// RSI
	rsi := c.RSI(klines, 14)
	if rsi != nil {
		report.Indicators = append(report.Indicators, &types.TechnicalIndicator{
			Name: "RSI",
			Values: map[string]float64{
				"RSI14": rsi.Values[lastIdx],
			},
			Signal: rsi.Signal,
		})
	}

	// KDJ
	kdj := c.KDJ(klines, 9, 3, 3)
	if kdj != nil {
		report.Indicators = append(report.Indicators, &types.TechnicalIndicator{
			Name: "KDJ",
			Values: map[string]float64{
				"K": kdj.K[lastIdx],
				"D": kdj.D[lastIdx],
				"J": kdj.J[lastIdx],
			},
			Signal: kdj.Signal,
		})
	}

	// BOLL
	boll := c.BOLL(klines, 20, 2)
	if boll != nil {
		report.Indicators = append(report.Indicators, &types.TechnicalIndicator{
			Name: "BOLL",
			Values: map[string]float64{
				"UPPER":  boll.Upper[lastIdx],
				"MIDDLE": boll.Middle[lastIdx],
				"LOWER":  boll.Lower[lastIdx],
			},
			Signal: boll.Signal,
		})

		// 计算支撑位和阻力位
		report.Support = boll.Lower[lastIdx]
		report.Resistance = boll.Upper[lastIdx]
	}

	// 综合判断趋势
	buyCount := 0
	sellCount := 0
	for _, ind := range report.Indicators {
		switch ind.Signal {
		case "buy", "oversold":
			buyCount++
		case "sell", "overbought":
			sellCount++
		}
	}

	if buyCount > sellCount+1 {
		report.Trend = "up"
		report.Signal = "buy"
	} else if sellCount > buyCount+1 {
		report.Trend = "down"
		report.Signal = "sell"
	} else {
		report.Trend = "sideways"
		report.Signal = "neutral"
	}

	// 生成摘要
	report.Summary = c.generateSummary(report, klines[lastIdx])

	return report
}

// generateSummary 生成分析摘要
func (c *Calculator) generateSummary(report *types.AnalysisReport, lastKline *types.KLine) string {
	var summary string

	switch report.Trend {
	case "up":
		summary = "技术面看多。"
	case "down":
		summary = "技术面看空。"
	default:
		summary = "技术面震荡。"
	}

	// 添加关键指标描述
	for _, ind := range report.Indicators {
		switch ind.Name {
		case "RSI":
			if rsi, ok := ind.Values["RSI14"]; ok {
				if rsi < 30 {
					summary += "RSI超卖区域，可能存在反弹机会。"
				} else if rsi > 70 {
					summary += "RSI超买区域，注意回调风险。"
				}
			}
		case "MACD":
			if ind.Signal == "buy" {
				summary += "MACD金叉，短期看多。"
			} else if ind.Signal == "sell" {
				summary += "MACD死叉，短期看空。"
			}
		}
	}

	if report.Support > 0 && report.Resistance > 0 {
		summary += fmt.Sprintf("支撑位: %.2f, 阻力位: %.2f。", report.Support, report.Resistance)
	}

	return summary
}
