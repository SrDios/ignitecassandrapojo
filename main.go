package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	_ "github.com/amsokol/ignite-go-client/sql"
	"github.com/iancoleman/strcase"
	"github.com/valyala/fasttemplate"
)

type table struct {
	tableName  string
	column     []column
	pk         column
	composedPK string
}

type column struct {
	name     string
	attrType string
}

var keyspace = "localhost"
var javaTypeMap = make(map[string]string)
var tempComposedPK string
var tempTable table
var listOfUniquePrimarys []string

func getInnerSubstring(str string, prefix string, suffix string) string {
	var beginIndex, endIndex int
	beginIndex = strings.Index(str, prefix)
	if beginIndex == -1 {
		beginIndex = 0
		endIndex = 0
	} else if len(prefix) == 0 {
		beginIndex = 0
		endIndex = strings.Index(str, suffix)
		if endIndex == -1 || len(suffix) == 0 {
			endIndex = len(str)
		}
	} else {
		beginIndex += len(prefix)
		endIndex = strings.Index(str[beginIndex:], suffix)
		if endIndex == -1 {
			if strings.Index(str, suffix) < beginIndex {
				endIndex = beginIndex
			} else {
				endIndex = len(str)
			}
		} else {
			if len(suffix) == 0 {
				endIndex = len(str)
			} else {
				endIndex += beginIndex
			}
		}
	}
	return str[beginIndex:endIndex]
}
func getTableName(element string) string {
	element = getInnerSubstring(element, keyspace+".", " (")
	return element
}

func main() {
	//Set MAP defaults
	javaTypeMap["int"] = "Integer"
	javaTypeMap["uuid"] = "UUID"
	javaTypeMap["text"] = "String"
	javaTypeMap["timestamp"] = "Date"
	javaTypeMap["float"] = "Float"
	javaTypeMap["boolean"] = "Boolean"
	javaTypeMap["decimal"] = "BigDecimal"
	/*if len(os.Args) < 2 {
		println("Paramter KEYSPACE required  -- go run main.go keyspace")
		os.Exit(3)
	}*/
	var tablesList []table
	//keyspace = os.Args[1]

	cmd := exec.Command("/home/user/Develop/git/apache-cassandra-3.11.2/bin/cqlsh", "localhost", "-e", "DESCRIBE somekeyspace")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	//fmt.Printf("%s\n", out.String())
	if err != nil {
		log.Fatal("Run: ", err)
	}
	outS := out.String()
	var tablenames []string
	tables := strings.Split(outS, "CREATE")
	for _, element := range tables {
		if strings.Contains(element, "TABLE") {
			//Get the table name
			name := getTableName(element)
			tablenames = append(tablenames, name)
			var table_ table
			primary := false
			table_.tableName = name

			//Get the attributes
			attr := getInnerSubstring(element, "(\n", "\n)")
			attrList := strings.Split(attr, "\n")
			for n, attribute := range attrList {
				//This is the last attribute, which will be a ComposedPK or not.
				if len(attrList)-1 == n {
					if strings.Contains(attribute, "PRIMARY KEY") {
						//We have a composedPK here
						tempComposedPK = attribute
						table_.composedPK = attribute
					} else {
						//We DON'T have a composedPK so is just a normal attr.
						attribute = strings.TrimLeft(attribute, " ")
						attribute = strings.TrimRight(attribute, " ")
						splitted := strings.Split(attribute, " ")
						var column_ column
						column_.name = splitted[0]
						column_.attrType = splitted[1]
						table_.column = append(table_.column, column_)
					}
				} else {
					//Remove last ,
					attribute = strings.TrimRight(attribute, ",")

					if strings.Contains(attribute, "PRIMARY KEY") {
						primary = true
					}
					//Trim left & right whitesapces
					attribute = strings.TrimLeft(attribute, " ")
					attribute = strings.TrimRight(attribute, " ")
					splitted := strings.Split(attribute, " ")
					var column_ column
					column_.name = splitted[0]
					column_.attrType = splitted[1]
					if primary {
						primary = false
						table_.pk.name = splitted[0]
						table_.pk.attrType = splitted[1]
						listOfUniquePrimarys = append(listOfUniquePrimarys, table_.tableName)

					}
					table_.column = append(table_.column, column_)

				}

			}
			if !contains(listOfUniquePrimarys, table_.tableName) {
				generateJavaPOJOPK(tempComposedPK, &table_)
			}
			tablesList = append(tablesList, table_)
		}
	}

	println("Number of tables:", len(tablesList))
	generateJavaPOJO(tablesList)
	generateXMLConfig(tablesList)
	generateStarter(tablesList)
}

func generateXMLConfig(tables []table) {

	for _, table := range tables {
		//Read XML template file
		inputBean, err := ioutil.ReadFile("./beanTemplate.xml")

		if err != nil {
			log.Fatalln(err)
		}
		template := string(inputBean[:])

		attribute := strings.Replace(table.composedPK, "PRIMARY KEY", "", -1)
		attribute = strings.TrimSpace(attribute)
		runes := []rune(attribute)
		// ... Convert back into a string from rune slice.
		substring := string(runes[0:2])

		mappingpartitionkey := ""
		mappingclusterkey := ""
		mappingpojovaluepersistence := ""

		if substring == "((" {
			//We have composed PK WITH partition Key
			attribute = strings.Replace(attribute, "(", "", 1)
			attribute = strings.TrimRight(attribute, ")")
			pksString := getInnerSubstring(attribute, "(", ")")
			pks := strings.Split(pksString, ",")
			cluster := strings.Replace(attribute, pksString, "", -1)
			cluster = strings.Replace(cluster, "(), ", "", -1)
			clusterKeys := strings.Split(cluster, ",")
			for _, col := range table.column {
				mappingpojovaluepersistence += "<field name=\"" + lowerInitial(strcase.ToCamel(col.name)) + "\" column=\"" + col.name + "\" />\n\t\t\t\t\t\t\t"
				if table.tableName == "current_rates_by_location" {
				}
			}
			for _, c := range clusterKeys {

				mappingclusterkey += "<field name=\"" + lowerInitial(strcase.ToCamel(c)) + "\" column=\"" + c + "\" />\n\t\t\t\t\t\t\t"
			}
			for _, pk := range pks {
			INNER:
				for _, col := range table.column {
					if strings.TrimSpace(col.name) == strings.TrimSpace(pk) {
						mappingpartitionkey += "<field name=\"" + lowerInitial(strcase.ToCamel(col.name)) + "\" column=\"" + col.name + "\" />\n\t\t\t\t\t\t\t"
						break INNER
					}
				}
			}

		} else {
			//We have PK WITH partition Key
			pksString := getInnerSubstring(attribute, "(", ")")
			pks := strings.Split(pksString, ",")
			for _, col := range table.column {
				mappingpojovaluepersistence += "<field name=\"" + lowerInitial(strcase.ToCamel(col.name)) + "\" column=\"" + col.name + "\" />\n\t\t\t\t\t\t\t"
			}
			for n, pk := range pks {

				if n > 0 {
					mappingclusterkey += "<field name=\"" + lowerInitial(strcase.ToCamel(pk)) + "\" column=\"" + pk + "\" />\n\t\t\t\t\t\t\t"
				}

			INNER2:
				for n, col := range table.column {
					if strings.TrimSpace(col.name) == strings.TrimSpace(pk) && n == 0 {
						mappingpartitionkey += "<field name=\"" + lowerInitial(strcase.ToCamel(col.name)) + "\" column=\"" + col.name + "\" />\n\t\t\t\t\t\t\t"
						break INNER2
					}
				}
			}
		}
		t := fasttemplate.New(template, "XXX_", "_XXX")
		prefix := ""
		columnane := "column=\"" + table.pk.name + "\""
		strategy := "PRIMITIVE"
		if javaTypeMap[table.pk.attrType] == "Date" || javaTypeMap[table.pk.attrType] == "UUID" {
			prefix = "java.util." + javaTypeMap[table.pk.attrType]
		} else if javaTypeMap[table.pk.attrType] == "BigDecimal" {
			prefix = "java.math." + javaTypeMap[table.pk.attrType]
		} else if table.pk.attrType == "POJO" {
			prefix = "com.fexco.brw.tables." + strings.Replace(table.pk.name, ".java", "", -1)
			strategy = "POJO"
			columnane = ""
		} else {
			prefix = "java.lang." + javaTypeMap[table.pk.attrType]
		}

		s := t.ExecuteString(map[string]interface{}{
			"TABLENAME":                   table.tableName,
			"JAVANAME":                    strcase.ToCamel(table.tableName),
			"TYPE":                        prefix,
			"COLUMN":                      columnane,
			"STRATEGY":                    strategy,
			"PRIMARYKEY":                  table.pk.name,
			"MAPPINGPK":                   mappingpartitionkey,
			"MAPPINGPOJOVALUEPERSISTENCE": mappingpojovaluepersistence,
			"MAPPINGCLUSTERKEY":           mappingclusterkey,
		})
		err = ioutil.WriteFile("./xmlBeans/"+table.tableName+".xml", []byte(s), 0644)
	}
	root := "./xmlBeans/"
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}
	beans := ""
	for _, file := range files {
		inputXML, _ := ioutil.ReadFile(file)
		beans += string(inputXML[:]) + "\n\n"
	}

	inputXML, _ := ioutil.ReadFile("./xmlTemplate.xml")
	template := string(inputXML[:])
	t := fasttemplate.New(template, "XXX_", "_XXX")
	s := t.ExecuteString(map[string]interface{}{
		"BEAN": beans,
	})

	_ = ioutil.WriteFile("./generated/client/"+"ignite-config"+".xml", []byte(s), 0644)

	inputXML, _ = ioutil.ReadFile("./xmlTemplateServer.xml")
	template = string(inputXML[:])
	t = fasttemplate.New(template, "XXX_", "_XXX")
	s = t.ExecuteString(map[string]interface{}{
		"BEAN": beans,
	})

	_ = ioutil.WriteFile("./generated/server/"+"ignite-config"+".xml", []byte(s), 0644)
}
func contains(s []string, e string) bool {

	for _, a := range s {
		if strings.TrimSpace(a) == strings.TrimSpace(e) {
			return true
		}
	}
	return false
}
func generateJavaPOJOPK(attribute string, table_ *table) {
	attribute = strings.Replace(attribute, "PRIMARY KEY", "", -1)
	attribute = strings.TrimSpace(attribute)
	runes := []rune(attribute)
	// ... Convert back into a string from rune slice.
	substring := string(runes[0:2])
	if substring == "((" {
		//We have composed PK WITH partition Key
		attribute = strings.Replace(attribute, "(", "", 1)
		attribute = strings.TrimRight(attribute, ")")
		pksString := getInnerSubstring(attribute, "(", ")")
		pks := strings.Split(pksString, ",")
		cluster := strings.Replace(attribute, pksString, "", -1)
		cluster = strings.Replace(cluster, "(), ", "", -1)
		clusterKeys := strings.Split(cluster, ",")
		println(cluster)
		fields := ""
		getter := ""
		setter := ""
		var coladded []string
		for _, pk := range pks {
			//INNER:
			for _, col := range table_.column {
				if strings.TrimSpace(col.name) == strings.TrimSpace(pk) {
					fields += "@AffinityKeyMapped\n"
					fields += "\tprivate " + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ";\n\n"
					getter += "\tpublic " + javaTypeMap[col.attrType] + " get" + strcase.ToCamel(col.name) + "() {\n"
					getter += "\t\treturn " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					getter += "\t}\n\n"
					setter += "\tpublic void" + " set" + strcase.ToCamel(col.name) + "(" + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ") {\n"
					setter += "\t\tthis." + lowerInitial(strcase.ToCamel(col.name)) + " = " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					setter += "\t}\n\n"
					//break INNER
				} else if contains(clusterKeys, strings.TrimSpace(col.name)) && !contains(coladded, strings.TrimSpace(col.name)) {
					fields += "\tprivate " + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ";\n\n"
					getter += "\tpublic " + javaTypeMap[col.attrType] + " get" + strcase.ToCamel(col.name) + "() {\n"
					getter += "\t\treturn " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					getter += "\t}\n\n"
					setter += "\tpublic void" + " set" + strcase.ToCamel(col.name) + "(" + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ") {\n"
					setter += "\t\tthis." + lowerInitial(strcase.ToCamel(col.name)) + " = " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					setter += "\t}\n\n"
					coladded = append(coladded, col.name)
				}
			}
		}
		input, _ := ioutil.ReadFile("./template.java")
		template := string(input[:])
		t := fasttemplate.New(template, "XXX_", "_XXX")
		s := t.ExecuteString(map[string]interface{}{
			"TABLENAME": strcase.ToCamel(table_.tableName) + "PK",
			"FIELDS":    fields,
			"GETTER":    getter,
			"SETTER":    setter,
		})
		_ = ioutil.WriteFile("./generated/PK/"+strcase.ToCamel(table_.tableName)+"PK.java", []byte(s), 0644)

	} else {

		//We have PK WITH partition Key
		pksString := getInnerSubstring(attribute, "(", ")")
		pks := strings.Split(pksString, ",")
		fields := ""
		getter := ""
		setter := ""
		for n, pk := range pks {
		INNER2:
			for _, col := range table_.column {
				if strings.TrimSpace(col.name) == strings.TrimSpace(pk) {
					if n == 0 {
						fields += "\t@AffinityKeyMapped\n"
					}
					fields += "\tprivate " + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ";\n\n"
					getter += "\tpublic " + javaTypeMap[col.attrType] + " get" + strcase.ToCamel(col.name) + "() {\n"
					getter += "\t\treturn " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					getter += "\t}\n\n"
					setter += "\tpublic void" + " set" + strcase.ToCamel(col.name) + "(" + javaTypeMap[col.attrType] + " " + lowerInitial(strcase.ToCamel(col.name)) + ") {\n"
					setter += "\t\tthis." + lowerInitial(strcase.ToCamel(col.name)) + " = " + lowerInitial(strcase.ToCamel(col.name)) + ";\n"
					setter += "\t}\n\n"
					break INNER2
				}
			}
		}
		input, _ := ioutil.ReadFile("./template.java")
		template := string(input[:])
		t := fasttemplate.New(template, "XXX_", "_XXX")
		s := t.ExecuteString(map[string]interface{}{
			"TABLENAME": strcase.ToCamel(table_.tableName) + "PK",
			"FIELDS":    fields,
			"GETTER":    getter,
			"SETTER":    setter,
		})
		_ = ioutil.WriteFile("./generated/PK/"+strcase.ToCamel(table_.tableName)+"PK.java", []byte(s), 0644)
	}
	table_.pk.name = strcase.ToCamel(table_.tableName) + "PK.java"
	table_.pk.attrType = "POJO"

}
func generateJavaPOJO(tables []table) {
	for _, table := range tables {
		//Read JAVA template file
		input, err := ioutil.ReadFile("./template.java")
		if err != nil {
			log.Fatalln(err)
		}
		fields := ""
		getter := ""
		setter := ""
		for _, column := range table.column {
			fields += "\t@QuerySqlField(index = false)\n"
			fields += "\tprivate " + javaTypeMap[column.attrType] + " " + lowerInitial(strcase.ToCamel(column.name)) + ";\n\n"
			getter += "\tpublic " + javaTypeMap[column.attrType] + " get" + strcase.ToCamel(column.name) + "() {\n"
			getter += "\t\treturn " + lowerInitial(strcase.ToCamel(column.name)) + ";\n"
			getter += "\t}\n\n"
			setter += "\tpublic void" + " set" + strcase.ToCamel(column.name) + "(" + javaTypeMap[column.attrType] + " " + lowerInitial(strcase.ToCamel(column.name)) + ") {\n"
			setter += "\t\tthis." + lowerInitial(strcase.ToCamel(column.name)) + " = " + lowerInitial(strcase.ToCamel(column.name)) + ";\n"
			setter += "\t}\n\n"
		}
		template := string(input[:])
		t := fasttemplate.New(template, "XXX_", "_XXX")
		s := t.ExecuteString(map[string]interface{}{
			"TABLENAME": strcase.ToCamel(table.tableName),
			"FIELDS":    fields,
			"GETTER":    getter,
			"SETTER":    setter,
		})
		err = ioutil.WriteFile("./generated/"+strcase.ToCamel(table.tableName)+".java", []byte(s), 0644)
	}
	println("Cya!")
}

func lowerInitial(str string) string {
	for i, v := range str {
		return string(unicode.ToLower(v)) + str[i+1:]
	}
	return ""
}

func generateStarter(tables []table) {
	cache := ""
	for _, tabl := range tables {
		cache += "\t\tignite.cache(\"" + strcase.ToCamel(tabl.tableName) + "\").loadCache(null);\n"
	}
	input, _ := ioutil.ReadFile("./starterTemplate.java")
	template := string(input[:])
	t := fasttemplate.New(template, "XXX_", "_XXX")
	s := t.ExecuteString(map[string]interface{}{
		"STARTCACHE": cache,
	})
	_ = ioutil.WriteFile("./Starter.java", []byte(s), 0644)
}

