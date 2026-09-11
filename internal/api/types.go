package api

type PutRequest struct {
	Value *string `json:"value"`
}
type JoinRequest struct {
	ID          string `json:"id"`
	GRPCAddress string `json:"grpc_address"`
	HTTPAddress string `json:"http_address"`
}
type RemoveRequest struct {
	ID string `json:"id"`
}
type ErrorResponse struct {
	Error string `json:"error"`
}
