package indicators

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// 创建测试K线数据
func createTestKLines(count int) []*types.KLine {
	klines := make([]*types.KLine, count)
	basePrice := 100.0
	baseTime := time.Now().AddDate(0, 0, -count)

	for i := 0; i < count; i++ {
		// 模拟价格波动
		change := (float64(i%10) - 5) * 0.5
		close := basePrice + change
		open := close - 0.5
		high := close + 1
		low := close - 1

		klines[i] = &types.KLine{
			Symbol:    "TEST",
			Period:    "1d",
			Timestamp: baseTime.AddDate(0, 0, i),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    int64(10000 + i*100),
			Amount:    float64(1000000 + i*10000),
		}
	}
	return klines
}

func TestMA(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	// 测试MA5
	ma5 := calc.MA(klines, 5)
	if ma5 == nil {
		t.Fatal("MA5 should not be nil")
	}
	if len(ma5) != len(klines) {
		t.Errorf("MA5 length should be %d, got %d", len(klines), len(ma5))
	}

	// 前4个应该是0或接近0
	if ma5[3] != 0 {
		t.Errorf("MA5[3] should be 0 before period is complete")
	}

	// 第5个应该有值
	if ma5[4] == 0 {
		t.Errorf("MA5[4] should have value")
	}

	// 测试MA10
	ma10 := calc.MA(klines, 10)
	if ma10 == nil {
		t.Fatal("MA10 should not be nil")
	}

	// 测试MA20
	ma20 := calc.MA(klines, 20)
	if ma20 == nil {
		t.Fatal("MA20 should not be nil")
	}
}

func TestEMA(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	// 测试EMA12
	ema12 := calc.EMA(klines, 12)
	if ema12 == nil {
		t.Fatal("EMA12 should not be nil")
	}
	if len(ema12) != len(klines) {
		t.Errorf("EMA12 length should be %d, got %d", len(klines), len(ema12))
	}

	// 第12个应该有值
	if ema12[11] == 0 {
		t.Errorf("EMA12[11] should have value")
	}

	// 测试EMA26
	ema26 := calc.EMA(klines, 26)
	if ema26 == nil {
		t.Fatal("EMA26 should not be nil")
	}
}

func TestMACD(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(60)

	macd := calc.MACD(klines, 12, 26, 9)
	if macd == nil {
		t.Fatal("MACD should not be nil")
	}

	if len(macd.DIF) != len(klines) {
		t.Errorf("MACD.DIF length should be %d, got %d", len(klines), len(macd.DIF))
	}

	if len(macd.DEA) != len(klines) {
		t.Errorf("MACD.DEA length should be %d, got %d", len(klines), len(macd.DEA))
	}

	if len(macd.MACD) != len(klines) {
		t.Errorf("MACD.MACD length should be %d, got %d", len(klines), len(macd.MACD))
	}

	// 信号应该是 buy/sell/neutral
	validSignals := map[string]bool{"buy": true, "sell": true, "neutral": true}
	if !validSignals[macd.Signal] {
		t.Errorf("MACD.Signal should be buy/sell/neutral, got %s", macd.Signal)
	}
}

func TestRSI(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	rsi := calc.RSI(klines, 14)
	if rsi == nil {
		t.Fatal("RSI should not be nil")
	}

	if len(rsi.Values) != len(klines) {
		t.Errorf("RSI.Values length should be %d, got %d", len(klines), len(rsi.Values))
	}

	// RSI值应该在0-100之间
	for i := 14; i < len(rsi.Values); i++ {
		if rsi.Values[i] < 0 || rsi.Values[i] > 100 {
			t.Errorf("RSI value should be between 0 and 100, got %f at index %d", rsi.Values[i], i)
		}
	}

	// 信号应该是 oversold/overbought/neutral
	validSignals := map[string]bool{"oversold": true, "overbought": true, "neutral": true}
	if !validSignals[rsi.Signal] {
		t.Errorf("RSI.Signal should be oversold/overbought/neutral, got %s", rsi.Signal)
	}
}

func TestKDJ(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	kdj := calc.KDJ(klines, 9, 3, 3)
	if kdj == nil {
		t.Fatal("KDJ should not be nil")
	}

	if len(kdj.K) != len(klines) {
		t.Errorf("KDJ.K length should be %d, got %d", len(klines), len(kdj.K))
	}

	if len(kdj.D) != len(klines) {
		t.Errorf("KDJ.D length should be %d, got %d", len(klines), len(kdj.D))
	}

	if len(kdj.J) != len(klines) {
		t.Errorf("KDJ.J length should be %d, got %d", len(klines), len(kdj.J))
	}

	// 信号应该是 buy/sell/neutral
	validSignals := map[string]bool{"buy": true, "sell": true, "neutral": true}
	if !validSignals[kdj.Signal] {
		t.Errorf("KDJ.Signal should be buy/sell/neutral, got %s", kdj.Signal)
	}
}

func TestBOLL(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	boll := calc.BOLL(klines, 20, 2)
	if boll == nil {
		t.Fatal("BOLL should not be nil")
	}

	if len(boll.Upper) != len(klines) {
		t.Errorf("BOLL.Upper length should be %d, got %d", len(klines), len(boll.Upper))
	}

	if len(boll.Middle) != len(klines) {
		t.Errorf("BOLL.Middle length should be %d, got %d", len(klines), len(boll.Middle))
	}

	if len(boll.Lower) != len(klines) {
		t.Errorf("BOLL.Lower length should be %d, got %d", len(klines), len(boll.Lower))
	}

	// 上轨应该大于中轨，中轨应该大于下轨
	for i := 19; i < len(klines); i++ {
		if boll.Upper[i] < boll.Middle[i] {
			t.Errorf("BOLL.Upper should be >= BOLL.Middle at index %d", i)
		}
		if boll.Middle[i] < boll.Lower[i] {
			t.Errorf("BOLL.Middle should be >= BOLL.Lower at index %d", i)
		}
	}

	// 信号应该是 buy/sell/neutral
	validSignals := map[string]bool{"buy": true, "sell": true, "neutral": true}
	if !validSignals[boll.Signal] {
		t.Errorf("BOLL.Signal should be buy/sell/neutral, got %s", boll.Signal)
	}
}

func TestWR(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	wr := calc.WR(klines, 14)
	if wr == nil {
		t.Fatal("WR should not be nil")
	}

	if len(wr.Values) != len(klines) {
		t.Errorf("WR.Values length should be %d, got %d", len(klines), len(wr.Values))
	}

	// WR值应该在0-100之间
	for i := 13; i < len(wr.Values); i++ {
		if wr.Values[i] < 0 || wr.Values[i] > 100 {
			t.Errorf("WR value should be between 0 and 100, got %f at index %d", wr.Values[i], i)
		}
	}
}

func TestCCI(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	cci := calc.CCI(klines, 14)
	if cci == nil {
		t.Fatal("CCI should not be nil")
	}

	if len(cci.Values) != len(klines) {
		t.Errorf("CCI.Values length should be %d, got %d", len(klines), len(cci.Values))
	}

	// 信号应该是 oversold/overbought/neutral
	validSignals := map[string]bool{"oversold": true, "overbought": true, "neutral": true}
	if !validSignals[cci.Signal] {
		t.Errorf("CCI.Signal should be oversold/overbought/neutral, got %s", cci.Signal)
	}
}

func TestATR(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	atr := calc.ATR(klines, 14)
	if atr == nil {
		t.Fatal("ATR should not be nil")
	}

	if len(atr) != len(klines) {
		t.Errorf("ATR length should be %d, got %d", len(klines), len(atr))
	}

	// ATR值应该大于0
	for i := 14; i < len(atr); i++ {
		if atr[i] <= 0 {
			t.Errorf("ATR value should be > 0, got %f at index %d", atr[i], i)
		}
	}
}

func TestOBV(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	obv := calc.OBV(klines)
	if obv == nil {
		t.Fatal("OBV should not be nil")
	}

	if len(obv) != len(klines) {
		t.Errorf("OBV length should be %d, got %d", len(klines), len(obv))
	}
}

func TestVOL(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(50)

	vol := calc.VOL(klines)
	if vol == nil {
		t.Fatal("VOL should not be nil")
	}

	if len(vol.Volume) != len(klines) {
		t.Errorf("VOL.Volume length should be %d, got %d", len(klines), len(vol.Volume))
	}

	if len(vol.MA5) != len(klines) {
		t.Errorf("VOL.MA5 length should be %d, got %d", len(klines), len(vol.MA5))
	}
}

func TestAnalyze(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(60)

	report := calc.Analyze(klines)
	if report == nil {
		t.Fatal("Analysis report should not be nil")
	}

	if len(report.Indicators) == 0 {
		t.Error("Report should have indicators")
	}

	// 检查包含关键指标
	indicatorNames := make(map[string]bool)
	for _, ind := range report.Indicators {
		indicatorNames[ind.Name] = true
	}

	expectedIndicators := []string{"MA", "MACD", "RSI", "KDJ", "BOLL"}
	for _, name := range expectedIndicators {
		if !indicatorNames[name] {
			t.Errorf("Report should contain %s indicator", name)
		}
	}

	// 趋势应该是 up/down/sideways
	validTrends := map[string]bool{"up": true, "down": true, "sideways": true}
	if !validTrends[report.Trend] {
		t.Errorf("Report.Trend should be up/down/sideways, got %s", report.Trend)
	}

	// 信号应该是 buy/sell/neutral
	validSignals := map[string]bool{"buy": true, "sell": true, "neutral": true}
	if !validSignals[report.Signal] {
		t.Errorf("Report.Signal should be buy/sell/neutral, got %s", report.Signal)
	}

	// 支撑位和阻力位应该大于0
	if report.Support <= 0 {
		t.Error("Report.Support should be > 0")
	}
	if report.Resistance <= 0 {
		t.Error("Report.Resistance should be > 0")
	}

	// 阻力位应该大于支撑位
	if report.Resistance <= report.Support {
		t.Error("Report.Resistance should be > Report.Support")
	}

	// 摘要应该非空
	if report.Summary == "" {
		t.Error("Report.Summary should not be empty")
	}
}

func TestInsufficientData(t *testing.T) {
	calc := NewCalculator()
	klines := createTestKLines(5) // 数据量不足

	// MA20需要至少20条数据
	ma20 := calc.MA(klines, 20)
	if ma20 != nil {
		t.Error("MA20 should be nil with insufficient data")
	}

	// MACD需要至少26条数据
	macd := calc.MACD(klines, 12, 26, 9)
	if macd != nil {
		t.Error("MACD should be nil with insufficient data")
	}

	// RSI需要至少15条数据
	rsi := calc.RSI(klines, 14)
	if rsi != nil {
		t.Error("RSI should be nil with insufficient data")
	}
}
