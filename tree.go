package web

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"vectors/utils"
)

const (
	StaticNode  NodeType = iota // static, should equal
	VariantNode                 // named node, match a non-/ is ok
	AnyNode                     // catch-all node, match any
	RegexpNode                  // regex node, should match

	AllType    ContentType = iota
	NumberType             // 数字内容
	CharType               // 字母内容
)

var (
	HttpMethods = []string{
		"GET",
		"POST",
		"HEAD",
		"DELETE",
		"PUT",
		"OPTIONS",
		"TRACE",
		"PATCH",
	}
)

type (
	NodeType    byte
	ContentType byte

	param struct {
		Name  string
		Value string
	}

	Params []param

	// 使用Sort 接口自动排序
	TOrder []*TNode

	TNode struct {
		Type        NodeType
		ContentType ContentType
		Children    TOrder
		//StaticChild  map[string]*TNode
		//VariantChild map[string]*TNode
		//RegexpChild  map[string]*TNode //[]*TNode
		Text  string // Path string /web/
		Path  string
		Route *TRoute
		//Left         string            // 左XX（
		//Middle       string
		//Right        string
		//Path2        string
		//IsLast bool
		//IsRoot bool

		Level  int // #动态Node排序等级 /.../ 之间的Nodes越多等级越高
		regexp *regexp.Regexp
	}

	TTree struct {
		Text string
		Root map[string]*TNode

		IgnoreCase bool

		//lock sync.RWMutex
	}
)

func (p *Params) Get(key string) string {
	for _, v := range *p {
		if v.Name == key {
			return v.Value
		}
	}
	return ""
}

func (p *Params) Set(key, value string) {
	for i, v := range *p {
		if v.Name == key {
			(*p)[i].Value = value
			return
		}
	}
}

func (p *Params) SetParams(params []param) {
	*p = params
}

func (self TOrder) Len() int {
	return len(self)
}

func (self TOrder) Swap(i, j int) {
	self[i], self[j] = self[j], self[i]
}

// static route will be put the first, so it will be match first.
// two static route, content longer is first.
func (self TOrder) Less(i, j int) bool {
	if self[i].Type == StaticNode {
		if self[j].Type == StaticNode {
			return len(self[i].Text) > len(self[j].Text)
		}
		return true
	}

	if self[j].Type == StaticNode {
		return false
	} else {
		return self[i].Level > self[j].Level
	}

	return i < j
}

func NewRouteTree() *TTree {
	lTree := &TTree{
		Root: make(map[string]*TNode),
	}
	for _, m := range HttpMethods {
		lTree.Root[m] = &TNode{
			Children: TOrder{},
		}
	}
	return lTree
}

// 解析Path为Node
/*   /:name1/:name2 /:name1-:name2 /(:name1)sss(:name2)
     /(*name) /(:name[0-9]+) /(type:name[a-z]+)
	Result: @ Nodes List
	        @ is it dyn route
*/
func (r *TTree) parsePath(aPath string) (nodes []*TNode, isDyn bool) {
	if aPath == "" {
		panic("echo: path cannot be empty")
	}
	if aPath[0] != '/' {
		aPath = "/" + aPath
	}

	var (
		i, j int // i 游标 J 上次标记

		bracket int
		level   int    // #Node的 排序等级
		target  *TNode // 记录等级的Node 一般为/ 开始的第一个动态
		node    *TNode
	)
	// 默认
	nodes = make([]*TNode, 0)
	isDyn = false
	l := len(aPath)
	//j = i - 1 // 当i==0时J必须小于它
	for ; i < l; i++ {
		switch aPath[i] {
		case '/':

			{ // 创建Text:'/' Node
				if bracket == 0 && i > j {
					//if aPath[j] == '/' {
					//	nodes = append(nodes, &TNode{Type: StaticNode, Text: string(aPath[j])})
					//}
					//j++
					nodes = append(nodes, &TNode{Type: StaticNode, Text: aPath[j:i]})
					j = i
				}

				//fmt.Println("/")
				// # 重置计数
				target = nil
				level = 0 // #开始计数
			}
		case '(':
			{
				//fmt.Println("(")
				bracket = 1
			}

		case ':':
			{
				//fmt.Println(":")
				var typ ContentType = AllType
				//fmt.Println(":", bracket, aPath[j:i-bracket])
				if aPath[i-1] == '(' { //#like (:var)
					nodes = append(nodes, &TNode{Type: StaticNode, Text: aPath[j : i-bracket]})
					bracket = 1
				} else {
					// #为变量区分数据类型
					str := aPath[j : i-bracket] // #like /abc1(string|upper:var)
					idx := strings.Index(str, "(")
					if idx == -1 {
						panic(fmt.Sprintf("expect a '(' near position %d~%d", j, i))
					}
					nodes = append(nodes, &TNode{Type: StaticNode, Text: str[:idx]})
					str = str[idx+1:]
					switch str {
					case "int":
						typ = NumberType
					case "string":
						typ = CharType
					default:
						typ = AllType
					}
					//fmt.Println("type:", typ)
					bracket = 1
				}

				j = i
				var (
					regex string
					start = -1
				)

				if bracket == 1 {
					// 开始记录Pos
					for ; i < l && ')' != aPath[i]; i++ { // 移动Pos到） 遇到正则字符标记起
						if start == -1 && utils.IsSpecialByte(aPath[i]) { // 如果是正则
							start = i
						}
					}
					if aPath[i] != ')' {
						panic("lack of )")
					}

					if start > -1 {
						regex = aPath[start:i] //正则内容
					}
				} else {
					i = i + 1
					for ; i < l && utils.IsAlnumByte(aPath[i]); i++ {
					}
				}

				if len(regex) > 0 { // 正则
					node = &TNode{Type: RegexpNode, regexp: regexp.MustCompile("(" + regex + ")"), Text: aPath[j : i-len(regex)]}
					nodes = append(nodes, node)
				} else { // 变量
					node = &TNode{Type: VariantNode, ContentType: typ, Text: aPath[j:i]}
					nodes = append(nodes, node)
				}

				isDyn = true    // #标记 Route 为动态
				i = i + bracket // #剔除")"字符 bracket=len(“)”)
				j = i

				// 当计数器遇到/或者Url末尾时将记录保存于Node中
				if target != nil && ((i == l) || (i != l && aPath[j+1] == '/')) {
					level++
					target.Level = level
					//fmt.Println("ok:", node.Text, target.Text, level)

					// # 重置计数
					target = nil
					level = 0
				}

				if i == l {
					return //nodes, isDyn
				}

				// #计数滴答
				// 放置在 i == l 后 确保表达式2比1多一个层级
				// @/(int:id1)-(:unique2)
				// @/(:id3)-(:unique3)/(:filename)
				if (i != l && aPath[j] != '/') || level != 0 {
					if level == 0 {
						target = node
					}

					level++
					//fmt.Println("leve:", node.Text, target.Text, level)
				}
			}
		case '*':
			{
				nodes = append(nodes, &TNode{Type: StaticNode, Text: aPath[j : i-bracket]})
				j = i
				//if bracket == 1 {
				//	for ; i < l && ')' == aPath[i]; i++ {
				//	}
				//} else {
				i = i + 1
				for ; i < l && utils.IsAlnumByte(aPath[i]); i++ {
				}
				//}
				nodes = append(nodes, &TNode{Type: AnyNode, Text: aPath[j:i]})
				isDyn = true    // 标记 Route 为动态
				i = i + bracket // bracket=len(“)”)
				j = i
				if i == l {
					return //nodes, isDyn
				}
			}

		default:
			{
				bracket = 0
			}
		}
	}

	nodes = append(nodes, &TNode{
		Type: StaticNode,
		Text: aPath[j:i],
	})

	//fmt.Println("lNodes", len(lNodes))
	return //nodes, isDyn
}

func (r *TTree) matchNode(aNode *TNode, aUrl string, aParams *Params) *TNode {
	var retnil bool
	if aNode.Type == StaticNode { // 静态节点

		if strings.HasPrefix(aUrl, aNode.Text) {
			//fmt.Println("J态", aUrl, " | ", aNode.Text[1:])
			if len(aUrl) == len(aNode.Text) {
				return aNode
			}

			for _, c := range aNode.Children {
				e := r.matchNode(c, aUrl[len(aNode.Text):], aParams)
				if e != nil {
					return e
				}
			}
		}

	} else if aNode.Type == AnyNode { // 全匹配节点
		//if len(aNode.Children) == 0 {
		//	*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
		//	return aNode
		//}
		//fmt.Println("Any态", aUrl, " | ", aNode.Text[1:])
		for _, c := range aNode.Children {
			idx := strings.LastIndex(aUrl, c.Text)
			//fmt.Println("LastIndex", aUrl, c.Text)
			if idx > -1 {
				h := r.matchNode(c, aUrl[idx:], aParams)
				if h != nil {
					*aParams = append(*aParams, param{aNode.Text[1:], aUrl[:idx]})
					return h
				}

			}
		}

		*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
		return aNode

	} else if aNode.Type == VariantNode { // 变量节点
		// # 消除path like /abc 的'/'
		idx := strings.IndexByte(aUrl, '/')
		//fmt.Println("D态", aUrl, " | ", aNode.Text[1:], idx)
		if idx == 0 { // #fix错误if idx > -1 {
			for _, c := range aNode.Children {
				h := r.matchNode(c, aUrl[idx:], aParams)
				if h != nil {
					/*fmt.Println("类型1", aUrl[:idx], aNode.ContentType)
					if !validType(aUrl[:idx], aNode.ContentType) {
						fmt.Println("错误类型", aUrl[:idx], aNode.ContentType)
						return nil
					}
					*/
					*aParams = append(*aParams, param{aNode.Text[1:], aUrl[:idx]})
					return h
				}
			}
			return nil
		}

		// 最底层Node
		//if len(aNode.Children) == 0 {
		//	*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
		//	return aNode
		//}
		//fmt.Println("Index", idx)
		for _, c := range aNode.Children {
			idx := strings.Index(aUrl, c.Text) // #匹配前面检索到的/之前的字符串
			//fmt.Println("Index", idx, aUrl, c.Text, aUrl[:idx])
			if idx > -1 {
				if len(aUrl[:idx]) > 1 && strings.Index(aUrl[:idx], "/") > -1 {
					retnil = true
					continue
				}

				//fmt.Println("类型2", aUrl[:idx], aNode.ContentType)
				if !validType(aUrl[:idx], aNode.ContentType) {
					//fmt.Println("错误类型", aUrl[:idx], aNode.ContentType)
					return nil
					//continue
				}
				h := r.matchNode(c, aUrl[idx:], aParams)
				if h != nil {
					*aParams = append(*aParams, param{aNode.Text[1:], aUrl[:idx]})
					return h
				}

				retnil = true
			}
		}

		if retnil {
			return nil
		}

		//fmt.Printf("动态", aUrl, aNode.Text[1:])
		*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
		return aNode

	} else if aNode.Type == RegexpNode { // 正则节点
		//if len(aNode.Children) == 0 && aNode.regexp.MatchString(aUrl) {
		//	*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
		//	return aNode
		//}
		idx := strings.IndexByte(aUrl, '/')
		if idx > -1 {
			if aNode.regexp.MatchString(aUrl[:idx]) {
				for _, c := range aNode.Children {
					h := r.matchNode(c, aUrl[idx:], aParams)
					if h != nil {
						*aParams = append(*aParams, param{aNode.Text[1:], aUrl[:idx]})
						return h
					}
				}
			}
			return nil
		}
		for _, c := range aNode.Children {
			idx := strings.Index(aUrl, c.Text)
			if idx > -1 && aNode.regexp.MatchString(aUrl[:idx]) {
				h := r.matchNode(c, aUrl[idx:], aParams)
				if h != nil {
					*aParams = append(*aParams, param{aNode.Text[1:], aUrl[:idx]})
					return h
				}

			}
		}

		if aNode.regexp.MatchString(aUrl) {
			*aParams = append(*aParams, param{aNode.Text[1:], aUrl})
			return aNode
		}

	}

	return nil
}

func (r *TTree) Match(method string, url string) (*TRoute, Params) {
	lRoot := r.Root[method]
	var lParams = make(Params, 0, strings.Count(url, "/"))
	for _, n := range lRoot.Children {
		e := r.matchNode(n, url, &lParams)
		if e != nil {
			////fmt.Println("matched:", e.Path, lParams)
			return e.Route, lParams
		}
	}
	////fmt.Println("not match:", url, lParams)
	return nil, nil
}

func validType(content string, typ ContentType) bool {
	switch typ {
	case NumberType:
		for i := 0; i < len(content); i++ {
			if !utils.IsDigitByte(content[i]) {
				return false
			}
		}
	case CharType:
		for i := 0; i < len(content); i++ {
			if !utils.IsAlphaByte(content[i]) {
				return false
			}
		}

	default:

	}

	return true
}

// validate parsed nodes, all non-static route should have static route children.
func validNodes(nodes []*TNode) bool {
	if len(nodes) == 0 {
		return false
	}
	var lastTp = nodes[0]
	for _, node := range nodes[1:] {
		if lastTp.Type != StaticNode && node.Type != StaticNode {
			return false
		}
		lastTp = node
	}
	return true
}

/*
 1# 获得被Hook的Route
pos: true 为插入Before 反之After

*/
func (self *TTree) __HookRoute(aMethod string, pos bool, aPath string, aRoute *TRoute) {

	// 解析并创建为Nodes的List形式

	lNodes, lIsDyn := self.parsePath(aPath)

	// 标记为动态路由
	aRoute.isDynRoute = lIsDyn // 即将Hook的新Route是动态地址

	// 记录动作名称
	aRoute.Action = lNodes[len(lNodes)].Text

	// 获得匹配
	lRoute, _ := self.Match(aMethod, aPath)
	lRoute.CombineController(aRoute)

	/*
		// 执行Route 的所有Controllers
		for action, ctrls := range lRoute.HookCtrl {
			lRoute.Ctrls[action] = append(lRoute.MainCtrl, ctrls...) // 合并Ctrl 到一起
		}
	*/
}

// 添加路由到Tree
func (self *TTree) AddRoute(aMethod, aPath string, aRoute *TRoute) {
	// 解析并创建为Nodes的List形式
	lNodes, lIsDyn := self.parsePath(aPath)

	// 标记为动态路由
	aRoute.isDynRoute = lIsDyn // 即将Hook的新Route是动态地址

	// 绑定Route到最后一个Node
	lNode := lNodes[len(lNodes)-1]
	aRoute.Action = lNode.Text // 赋值Action
	lNode.Route = aRoute
	lNode.Path = aPath
	// 验证合法性
	if !validNodes(lNodes) {
		panic(fmt.Sprintln("express", aPath, "is not supported"))
	}
	////fmt.Println("self.Root", self.Root, aMethod, lNodes)
	// 插入该节点到Tree
	self.addnodes(aMethod, lNodes, false)
	//r.printTrees()

	//aRoute.Ctrls[aRoute.Action] = aRoute.MainCtrl // 合并Ctrl 到一起
}

func (self *TTree) conbine(aDes, aSrc *TNode) {
	var lNode *TNode

	// 是否目标Node有该Node
	for _, node := range aDes.Children {
		if node.Equal(aSrc) {
			lNode = node
		}
	}
	// 如果:无该Node直接添加完成所有工作
	// 或者:遍历添加所有没有的新Node
	if lNode == nil {
		aDes.Children = append(aDes.Children, aSrc)
		return
	} else {
		if lNode.Type == RegexpNode {

		}

		if aSrc.Route != nil {
			if lNode.Route == nil {
				lNode.Route = aSrc.Route
			} else {
				// 叠加合并Controller
				lNode.Route.CombineController(aSrc.Route)
			}
		}

		// 合并子节点
		for _, _node := range aSrc.Children {
			self.conbine(lNode, _node)
		}
	}
}

func (self *TTree) Conbine(aTree *TTree) *TTree {
	for method, node := range self.Root {
		//for _, dnode := range node.Children {
		for _, snode := range aTree.Root[method].Children {
			self.conbine(node, snode)
		}
		//}
	}
	return self
}

// add node nodes[i] to parent node p
func (self *TNode) addnode(aParent *TNode, aNodes []*TNode, i int, aIsHook bool) *TNode {
	if len(aParent.Children) == 0 {
		aParent.Children = make([]*TNode, 0)
	}

	// 如果:找到[已经注册]的分支节点则从该节继续[查找/添加]下一个节点
	for _, n := range aParent.Children {
		if n.Equal(aNodes[i]) {
			// 如果:插入的节点层级已经到末尾,则为该节点注册路由
			if i == len(aNodes)-1 {
				// 原始路由会被替换
				if aIsHook {
					n.Route.CombineController(aNodes[i].Route)
				} else {
					n.Route = aNodes[i].Route
				}
			}
			return n
		}
	}

	// 如果:该节点没有对应分支则插入同级的aNodes为新的分支
	aParent.Children = append(aParent.Children, aNodes[i])
	sort.Sort(aParent.Children)
	return aNodes[i]
}

// add nodes to trees
func (self *TTree) addnodes(aMethod string, aNodes []*TNode, aIsHook bool) {
	//fmt.Println("self.Root", self.Root)
	// 获得对应方法[POST,GET...]
	cn := self.Root[aMethod]
	var p *TNode = cn // 复制方法对应的Root

	// 层级插入Nodes的Node到Root
	for idx, _ := range aNodes {
		p = cn.addnode(p, aNodes, idx, aIsHook)
	}
}

func printNode(i int, node *TNode) {
	for _, c := range node.Children {
		for j := 0; j < i; j++ { // 空格距离ss
			fmt.Print("  ")
		}
		if i > 1 {
			fmt.Print("┗", "  ")
		}

		fmt.Printf(`%s<lv:%d,%v>`, c.Text, c.Level, c.ContentType)
		if c.Route != nil {
			fmt.Print("<*>")
		}
		//if !reflect.DeepEqual(c.Route, TRoute{}) {
		if c.Route != nil {
			//fmt.Print("  ", c.Route.HandleType.String())
			//fmt.Printf("  %p", c.handle.method.Interface())
		}
		fmt.Println()
		printNode(i+1, c)
	}
}

func (self *TTree) PrintTrees() {
	for _, method := range HttpMethods {
		if len(self.Root[method].Children) > 0 {
			fmt.Println(method)
			printNode(1, self.Root[method])
			fmt.Println()
		}
	}
}

func (self *TNode) Equal(o *TNode) bool {
	if self.Type != o.Type || self.Text != o.Text {
		return false
	}
	return true
}

/*
TODO
实现前面加类型
'/web/content/<string:xmlid>',
'/web/content/<string:xmlid>/<string:filename>',
'/web/content/<int:id>',
'/web/content/<int:id>/<string:filename>',
'/web/content/<int:id>-<string:unique>',
'/web/content/<int:id>-<string:unique>/<string:filename>',
'/web/content/<string:model>/<int:id>/<string:field>',
'/web/content/<string:model>/<int:id>/<string:field>/<string:filename>'
*/
