package main
import ("context";"fmt";"github.com/1node/airon/internal/config";"github.com/1node/airon/internal/mcp")
func main() {
  cfg, err := config.Load("examples/full/airon.yaml")
  if err != nil { panic(err) }
  s := cfg.MCPServers[0]
  c, err := mcp.Connect(context.Background(), mcp.ServerConfig{Name:s.Name,Transport:s.Transport,Command:s.Command,Args:s.Args})
  if err != nil { panic(err) }
  defer c.Close()
  tools, _ := c.ListTools(context.Background())
  fmt.Println("args[2]=", s.Args[2]); fmt.Println("tools", len(tools))
}
