package control

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/canonical/microcluster/internal/rest/client"
	"github.com/canonical/microcluster/internal/rest/types"
)

// RunSQL executes a sql command against the cell/region database. It can also be used to dump the database schema.
func (c *CmdControl) RunSQL(cmd *cobra.Command, args []string) error {
	dir, err := c.GetStateDir()
	if err != nil {
		return err
	}

	if len(args) != 1 {
		err := cmd.Help()
		if err != nil {
			return fmt.Errorf("Unable to load help: %w", err)
		}

		if len(args) == 0 {
			return nil
		}
	}

	query := args[0]
	if query == "-" {
		// Read from stdin
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("Failed to read from stdin: %w", err)
		}

		query = string(bytes)
	}

	d, err := client.New(dir.ControlSocket(), nil, nil, false)
	if err != nil {
		return err
	}

	if query == ".dump" || query == ".schema" {
		dump, err := d.GetSQL(context.Background(), query == ".schema")
		if err != nil {
			return fmt.Errorf("failed to parse dump response: %w", err)
		}

		fmt.Printf(dump.Text)
		return nil
	}

	data := types.SQLQuery{
		Query: query,
	}

	batch, err := d.PostSQL(context.Background(), data)
	if err != nil {
		return err
	}

	for i, result := range batch.Results {
		if len(batch.Results) > 1 {
			fmt.Printf("=> Query %d:\n\n", i)
		}

		if result.Type == "select" {
			sqlPrintSelectResult(result)
		} else {
			fmt.Printf("Rows affected: %d\n", result.RowsAffected)
		}

		if len(batch.Results) > 1 {
			fmt.Printf("\n")
		}
	}
	return nil
}

func sqlPrintSelectResult(result types.SQLResult) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeader(result.Columns)
	for _, row := range result.Rows {
		data := []string{}
		for _, col := range row {
			data = append(data, fmt.Sprintf("%v", col))
		}

		table.Append(data)
	}

	table.Render()
}
