package beater

import (
	"fmt"
	"time"
	"strings"
	"strconv"
	"regexp"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/jeremyweader/cmkbeat/config"
	"github.com/jeremyweader/go-livestatus"
)

type Cmkbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	config := config.DefaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Cmkbeat{
		done: make(chan struct{}),
		config: config,
	}
	return bt, nil
}

func (bt *Cmkbeat) Run(b *beat.Beat) error {
	
	if len(bt.config.Cmkhost) < 1 {
		return fmt.Errorf("Error: Invalid cmkhost config \"%s\"", bt.config.Cmkhost)
	}
	if len(bt.config.Query) < 1 {
		return fmt.Errorf("Error: Invalid query config \"%s\"", bt.config.Query)
	}
	if len(bt.config.Columns) < 1 {
		return fmt.Errorf("Error: Invalid columns config \"%s\"", bt.config.Columns)
	}
	if bt.config.Metrics == true {
		var pd bool = false
		for _, v := range bt.config.Columns {
			if v == "perf_data" {
				pd = true
			}
		}
		if pd == false {
			return fmt.Errorf("Error: Metrics require searching for the perf_data column.")
		}
	}
	
	logp.Info("------Config-------")
	logp.Info("Host: %s", bt.config.Cmkhost)
	logp.Info("Query: %s", bt.config.Query)
	logp.Info("Columns: %s", bt.config.Columns)
	logp.Info("Filter: %s", bt.config.Filter)
	logp.Info("Metrics: %t", bt.config.Metrics)
	logp.Info("--------------")

	var err error
	bt.client, err = b.Publisher.Connect()

	if err != nil {
		return err
	}

	ticker := time.NewTicker(bt.config.Period)

	for {
		select {
			case <-bt.done:
				return nil
			case <-ticker.C:
		}

		err := bt.lsQuery(bt.config.Cmkhost, b.Info.Name)
		if err != nil {
			logp.Warn("Error executing query: %s", err)
			return err
		}
	}
}

func (bt *Cmkbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}

func (bt *Cmkbeat) lsQuery(lshost string, beatname string) error {

    start := time.Now()
	
	var host string = lshost
	var query string = bt.config.Query
	var columns []string = bt.config.Columns
	var filter []string = bt.config.Filter
	var metrics bool = bt.config.Metrics
	
    l := livestatus.NewLivestatus("tcp", host)
    q := l.Query(query)
    q.Columns(columns)
	
	if len(filter) > 0 {
		for _, f := range filter {
			if strings.HasPrefix(f, "And") {
				and, _ := strconv.Atoi(strings.TrimPrefix(f, "And: "))
				q.And(and)
			} else if strings.HasPrefix(f, "Or") {
				or, _ := strconv.Atoi(strings.TrimPrefix(f, "Or: "))
				q.Or(or)
			} else {
					q.Filter(f)
			}
		}
	}

    resp, err := q.Exec()
    if err != nil {
		return err
    }

	numRecords := 0

    for _, r := range resp.Records {
			
		event := beat.Event {
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"type":		beatname,
			},
		}
		
		var colData map[string]string
		colData = make(map[string]string)
		for _, c := range columns {
			var data interface{}
			data, err = r.Get(c)
			if err != nil {
				logp.Warn("Problem parsing response fields: %s", err)
			}
			if strData, ok := data.(string); ok {
				colData[c] = strData
				event.Fields[c] = strData
			} else {
				strData := fmt.Sprint(data)
				colData[c] = strData
				event.Fields[c] = strData
			}
		}
		
		if metrics {
                    var allow bool = true
                    if len(bt.config.MetricsAllow) > 0 {
                        allow = false
                        for _, a := range bt.config.MetricsAllow {
                            if a == colData["display_name"] {
                                logp.Info("Allowing metric: %s", a)
                                allow = true
                            }
                        }
                    }
                    if len(bt.config.MetricsBlock) > 0 {
                        for _, a := range bt.config.MetricsBlock {
                            if a == colData["display_name"] {
                                logp.Info("Blocking metric: %s", a)
                                allow = false
                            }
                        }
                    }
                    if allow {
                        serviceMap := common.MapStr{}
                        var perf_data string
                        perf_data = colData["perf_data"]
                        var sName string = colData["display_name"]
                        var uName string = strings.Replace(sName, " ", "_", -1)
                        if len(perf_data) > 0 {
                            var perfDataSplit []string
    
                            perfDataSplit = strings.Split(perf_data, " ")
                            for _, perfObj := range perfDataSplit {
                                var perfObjSplit []string
                                var dataSplit []string
                                if len(perfObj) > 0 {
                                    perfObjSplit = strings.Split(perfObj, "=")
                                    if len(perfObjSplit) == 2 {
                                        item := perfObjSplit[0]
                                        data := perfObjSplit[1]
                                        item_warn := item + "_warn"
                                        item_crit := item + "_crit"
                                        if len(data) > 0 {
                                            var num string
                                            var warn string = ""
                                            var crit string = ""
                                            if strings.Contains(data, ";") {
                                                dataSplit = strings.Split(data, ";")
                                                dsLen := len(dataSplit)
                                        
                                                // Current value
                                                if dsLen >= 1 {
                                                    if len(dataSplit[0]) > 0 {
                                                        re := regexp.MustCompile("[0-9\\.]+")
                                                        num = re.FindString(dataSplit[0])
                                                    }
                                                }
                                                // Warn value
                                                if dsLen >= 2 {
                                                    if len(dataSplit[1]) > 0 {
                                                        re := regexp.MustCompile("[0-9\\.]+")
                                                        warn = re.FindString(dataSplit[1])
                                                    }
                                                }
                                                // Crit value
                                                if dsLen >= 3 {
                                                    if len(dataSplit[2]) > 0 {
                                                        re := regexp.MustCompile("[0-9\\.]+")
                                                        crit = re.FindString(dataSplit[2])
                                                    }
                                                }
                                            } else {
                                                re := regexp.MustCompile("[0-9\\.]+")
                                                num = re.FindString(data)
                                            }
                                            if len(warn) > 0 {
                                                mItem := common.MapStr{
                                                    item:      num,
                                                    item_warn: warn,
                                                    item_crit: crit,
                                                }
                                                serviceMap = common.MapStrUnion(serviceMap, mItem)
                                            } else {
                                                mItem := common.MapStr{
                                                    item:      num,
                                                }
                                                serviceMap = common.MapStrUnion(serviceMap, mItem)
                                            }
                                        } 
                                    }
                                } 
                            }
                            event.Fields["metrics"] = common.MapStr{
                                uName:  serviceMap,
                            }
                        }
                    }
		} 
		bt.client.Publish(event)
		numRecords++
    }
    elapsed := time.Since(start)
    logp.Info("%v events submitted in %s.", numRecords, elapsed)
    return nil
}

