package dto

type UserList struct {
	Items []UserProfile `json:"items"`
	Total int           `json:"total"`
}

type ModelMetrics struct {
	MAE       float64 `json:"mae"`
	RMSE      float64 `json:"rmse"`
	TrainSize int     `json:"train_size"`
	TestSize  int     `json:"test_size"`
}

type MLMetrics struct {
	ShortTerm ModelMetrics `json:"short_term"`
	LongTerm  ModelMetrics `json:"long_term"`
}
