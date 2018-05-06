package galera

import (
	"database/sql"
	"errors"

	"github.com/docker/docker/client"
	// mysql client library
	_ "github.com/go-sql-driver/mysql"
)

const (
	// ErrNoNodeFound throws while there there is no node found with given id
	ErrNoNodeFound string = "No node found with given id"
)

// Cluster struct encapsulates informations about cluster like nodes in cluster
type Cluster struct {
	Name          string `json:"name"`
	Nodes         []Node `json:"nodes"`
	Client        *client.Client
	ConnectedNode string `json:"connected_node"`
	DB            *sql.DB
}

// NewCluster creates a new clusten instance (Constructor like function)
func NewCluster() (*Cluster, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Cluster{
		Client: cli,
	}, nil

}

// GetCluster gets all cluster details
func (c *Cluster) GetCluster() error {
	nodes, err := GetNodes(c.Client)
	if err != nil {
		return err
	}
	c.Nodes = nodes
	if len(nodes) == 0 {
		return nil
	}
	var selectedNode *Node
	for _, node := range nodes {
		if node.Active {
			selectedNode = &node
			break
		}
	}

	if selectedNode == nil {
		return nil
	}
	c.ConnectedNode = selectedNode.ContainerID
	c.DB, err = sql.Open("mysql", selectedNode.GetDBConnectionString())
	return err
}

// AddNode adds node to the cluster
func (c *Cluster) AddNode(name string) error {
	node := NewNode(name)
	return node.CreateNode(c.Client, c.Nodes[0].IP)
}

// Query will run query on selected cluster
func (c *Cluster) Query(query string) ([]map[string]string, error) {

	rows, err := c.DB.Query(query)
	if err != nil {
		return nil, err
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rowsToMap(rows)

}

func (c *Cluster) getNodeByContainerID(id string) *Node {
	var selectedNode *Node
	for _, node := range c.Nodes {
		if node.ContainerID == id {
			selectedNode = &node
			break
		}
	}
	return selectedNode
}

// StartNode will start a node in stopped state
func (c *Cluster) StartNode(id string) error {
	selectedNode := c.getNodeByContainerID(id)
	if selectedNode == nil {
		return errors.New(ErrNoNodeFound)
	}
	return selectedNode.StartNode(c.Client)
}

// StopNode will stop an running node
func (c *Cluster) StopNode(id string) error {
	selectedNode := c.getNodeByContainerID(id)
	if selectedNode == nil {
		return errors.New(ErrNoNodeFound)
	}
	return selectedNode.StopNode(c.Client)
}

// SwitchDBConnection will switch db connection to given node
func (c *Cluster) SwitchDBConnection(id string) error {
	err := c.DB.Close()
	if err != nil {
		return err
	}
	selectedNode := c.getNodeByContainerID(id)
	if selectedNode == nil {
		return errors.New(ErrNoNodeFound)
	}
	c.DB, err = sql.Open("mysql", selectedNode.GetDBConnectionString())
	c.ConnectedNode = selectedNode.ContainerID
	return err
}

// rowsToMap converts SQL row to string map, which is easier to convert to JSON
func rowsToMap(rows *sql.Rows) (results []map[string]string, err error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	valPointers := make([][]byte, len(columns))
	vals := make([]interface{}, len(columns))

	for rowIndex := range valPointers {
		vals[rowIndex] = &valPointers[rowIndex]
	}

	for rows.Next() {
		err := rows.Scan(vals...)
		if err != nil {
			return nil, err
		}
		res := make(map[string]string)

		for colIndex := range columns {
			res[columns[colIndex]] = string(valPointers[colIndex])
		}
		results = append(results, res)
	}
	return results, nil

}
