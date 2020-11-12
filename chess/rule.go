/**
 * 中国象棋
 * Designed by wqh, Version: 1.0
 * Copyright (C) 2020 www.wangqianhong.com
 * 象棋规则
 */

package chess

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"math/rand"
)

//RC4Struct RC4密码流生成器
type RC4Struct struct {
	s    [256]int
	x, y int
}

//initZero 用空密钥初始化密码流生成器
func (r *RC4Struct) initZero() {
	j := 0
	for i := 0; i < 256; i++ {
		r.s[i] = i
	}
	for i := 0; i < 256; i++ {
		j = (j + r.s[i]) & 255
		r.s[i], r.s[j] = r.s[j], r.s[i]
	}
}

//nextByte 生成密码流的下一个字节
func (r *RC4Struct) nextByte() uint32 {
	r.x = (r.x + 1) & 255
	r.y = (r.y + r.s[r.x]) & 255
	r.s[r.x], r.s[r.y] = r.s[r.y], r.s[r.x]
	return uint32(r.s[(r.s[r.x]+r.s[r.y])&255])
}

//nextLong 生成密码流的下四个字节
func (r *RC4Struct) nextLong() uint32 {
	uc0 := r.nextByte()
	uc1 := r.nextByte()
	uc2 := r.nextByte()
	uc3 := r.nextByte()
	return uc0 + (uc1 << 8) + (uc2 << 16) + (uc3 << 24)
}

//ZobristStruct Zobrist结构
type ZobristStruct struct {
	dwKey   uint32
	dwLock0 uint32
	dwLock1 uint32
}

//initZero 用零填充Zobrist
func (z *ZobristStruct) initZero() {
	z.dwKey, z.dwLock0, z.dwLock1 = 0, 0, 0
}

//initRC4 用密码流填充Zobrist
func (z *ZobristStruct) initRC4(rc4 *RC4Struct) {
	z.dwKey = rc4.nextLong()
	z.dwLock0 = rc4.nextLong()
	z.dwLock1 = rc4.nextLong()
}

//xor1 执行XOR操作
func (z *ZobristStruct) xor1(zobr *ZobristStruct) {
	z.dwKey ^= zobr.dwKey
	z.dwLock0 ^= zobr.dwLock0
	z.dwLock1 ^= zobr.dwLock1
}

//xor2 执行XOR操作
func (z *ZobristStruct) xor2(zobr1, zobr2 *ZobristStruct) {
	z.dwKey ^= zobr1.dwKey ^ zobr2.dwKey
	z.dwLock0 ^= zobr1.dwLock0 ^ zobr2.dwLock0
	z.dwLock1 ^= zobr1.dwLock1 ^ zobr2.dwLock1
}

//Zobrist Zobrist表
type Zobrist struct {
	Player *ZobristStruct          //走子方
	Table  [14][256]*ZobristStruct //所有棋子
}

//initZobrist 初始化Zobrist表
func (z *Zobrist) initZobrist() {
	rc4 := &RC4Struct{}
	rc4.initZero()
	z.Player.initRC4(rc4)
	for i := 0; i < 14; i++ {
		for j := 0; j < 256; j++ {
			z.Table[i][j] = &ZobristStruct{}
			z.Table[i][j].initRC4(rc4)
		}
	}
}

//MoveStruct 历史走法信息
type MoveStruct struct {
	ucpcCaptured int  //是否吃子
	ucbCheck     bool //是否将军
	wmv          int  //走法
	dwKey        uint32
}

//set 设置
func (m *MoveStruct) set(mv, pcCaptured int, bCheck bool, dwKey uint32) {
	m.wmv = mv
	m.ucpcCaptured = pcCaptured
	m.ucbCheck = bCheck
	m.dwKey = dwKey
}

//PositionStruct 局面结构
type PositionStruct struct {
	sdPlayer    int                   //轮到谁走，0=红方，1=黑方
	vlRed       int                   //红方的子力价值
	vlBlack     int                   //黑方的子力价值
	nDistance   int                   //距离根节点的步数
	nMoveNum    int                   //历史走法数
	ucpcSquares [256]int              //棋盘上的棋子
	mvsList     [MaxMoves]*MoveStruct //历史走法信息列表
	zobr        *ZobristStruct        //走子方zobrist校验码
	zobrist     *Zobrist              //所有棋子zobrist校验码
	search      *Search
}

//NewPositionStruct 初始化棋局
func NewPositionStruct() *PositionStruct {
	p := &PositionStruct{
		zobr: &ZobristStruct{
			dwKey:   0,
			dwLock0: 0,
			dwLock1: 0,
		},
		zobrist: &Zobrist{
			Player: &ZobristStruct{
				dwKey:   0,
				dwLock0: 0,
				dwLock1: 0,
			},
		},
		search: &Search{},
	}
	if p == nil {
		return nil
	}

	for i := 0; i < MaxMoves; i++ {
		tmpMoveStruct := &MoveStruct{}
		p.mvsList[i] = tmpMoveStruct
	}

	for i := 0; i < HashSize; i++ {
		p.search.hashTable[i] = &HashItem{}
	}

	p.zobrist.initZobrist()
	return p
}

//loadBook 加载开局库
func (p *PositionStruct) loadBook() bool {
	file, err := os.Open("./res/book.dat")
	if err != nil {
		fmt.Print(err)
		return false
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	if reader == nil {
		return false
	}

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				fmt.Print(err)
				return false
			}
		}
		tmpLine := string(line)
		tmpResult := strings.Split(tmpLine, ",")
		if len(tmpResult) == 3 {
			tmpItem := &BookItem{}
			tmpdwLock, err := strconv.ParseUint(tmpResult[0], 10, 32)
			if err != nil {
				fmt.Print(err)
				continue
			}
			tmpItem.dwLock = uint32(tmpdwLock)
			tmpwmv, err := strconv.ParseInt(tmpResult[1], 10, 32)
			if err != nil {
				fmt.Print(err)
				continue
			}
			tmpItem.wmv = int(tmpwmv)
			tmpwvl, err := strconv.ParseInt(tmpResult[2], 10, 32)
			if err != nil {
				fmt.Print(err)
				continue
			}
			tmpItem.wvl = int(tmpwvl)

			p.search.BookTable = append(p.search.BookTable, tmpItem)
		}
	}
	return true
}

//clearBoard 清空棋盘
func (p *PositionStruct) clearBoard() {
	p.sdPlayer, p.vlRed, p.vlBlack, p.nDistance = 0, 0, 0, 0
	for i := 0; i < 256; i++ {
		p.ucpcSquares[i] = 0
	}
	p.zobr.initZero()
}

//setIrrev 清空(初始化)历史走法信息
func (p *PositionStruct) setIrrev() {
	p.mvsList[0].set(0, 0, p.checked(), p.zobr.dwKey)
	p.nMoveNum = 1
}

//startup 初始化棋盘
func (p *PositionStruct) startup() {
	p.clearBoard()
	pc := 0
	for sq := 0; sq < 256; sq++ {
		pc = cucpcStartup[sq]
		if pc != 0 {
			p.addPiece(sq, pc)
		}
	}
	p.setIrrev()
}

//changeSide 交换走子方
func (p *PositionStruct) changeSide() {
	p.sdPlayer = 1 - p.sdPlayer
	p.zobr.xor1(p.zobrist.Player)
}

//addPiece 在棋盘上放一枚棋子
func (p *PositionStruct) addPiece(sq, pc int) {
	p.ucpcSquares[sq] = pc
	//红方加分，黑方(注意"cucvlPiecePos"取值要颠倒)减分
	if pc < 16 {
		p.vlRed += cucvlPiecePos[pc-8][sq]
		p.zobr.xor1(p.zobrist.Table[pc-8][sq])
	} else {
		p.vlBlack += cucvlPiecePos[pc-16][squareFlip(sq)]
		p.zobr.xor1(p.zobrist.Table[pc-9][sq])
	}
}

//delPiece 从棋盘上拿走一枚棋子
func (p *PositionStruct) delPiece(sq, pc int) {
	p.ucpcSquares[sq] = 0
	//红方减分，黑方(注意"cucvlPiecePos"取值要颠倒)加分
	if pc < 16 {
		p.vlRed -= cucvlPiecePos[pc-8][sq]
		p.zobr.xor1(p.zobrist.Table[pc-8][sq])
	} else {
		p.vlBlack -= cucvlPiecePos[pc-16][squareFlip(sq)]
		p.zobr.xor1(p.zobrist.Table[pc-9][sq])
	}
}

//evaluate 局面评价函数
func (p *PositionStruct) evaluate() int {
	if p.sdPlayer == 0 {
		return p.vlRed - p.vlBlack + AdvancedValue
	}

	return p.vlBlack - p.vlRed + AdvancedValue
}

//inCheck 是否被将军
func (p *PositionStruct) inCheck() bool {
	return p.mvsList[p.nMoveNum-1].ucbCheck
}

//captured 上一步是否吃子
func (p *PositionStruct) captured() bool {
	return p.mvsList[p.nMoveNum-1].ucpcCaptured != 0
}

//movePiece 搬一步棋的棋子
func (p *PositionStruct) movePiece(mv int) int {
	sqSrc := src(mv)
	sqDst := dst(mv)
	pcCaptured := p.ucpcSquares[sqDst]
	if pcCaptured != 0 {
		p.delPiece(sqDst, pcCaptured)
	}
	pc := p.ucpcSquares[sqSrc]
	p.delPiece(sqSrc, pc)
	p.addPiece(sqDst, pc)
	return pcCaptured
}

//undoMovePiece 撤消搬一步棋的棋子
func (p *PositionStruct) undoMovePiece(mv, pcCaptured int) {
	sqSrc := src(mv)
	sqDst := dst(mv)
	pc := p.ucpcSquares[sqDst]
	p.delPiece(sqDst, pc)
	p.addPiece(sqSrc, pc)
	if pcCaptured != 0 {
		p.addPiece(sqDst, pcCaptured)
	}
}

//makeMove 走一步棋
func (p *PositionStruct) makeMove(mv int) bool {
	dwKey := p.zobr.dwKey
	pcCaptured := p.movePiece(mv)
	if p.checked() {
		p.undoMovePiece(mv, pcCaptured)
		return false
	}
	p.changeSide()
	p.mvsList[p.nMoveNum].set(mv, pcCaptured, p.checked(), dwKey)
	p.nMoveNum++
	p.nDistance++
	return true
}

//undoMakeMove 撤消走一步棋
func (p *PositionStruct) undoMakeMove() {
	p.nDistance--
	p.nMoveNum--
	p.changeSide()
	p.undoMovePiece(p.mvsList[p.nMoveNum].wmv, p.mvsList[p.nMoveNum].ucpcCaptured)
}

//nullMove 走一步空步
func (p *PositionStruct) nullMove() {
	dwKey := p.zobr.dwKey
	p.changeSide()
	p.mvsList[p.nMoveNum].set(0, 0, false, dwKey)
	p.nMoveNum++
	p.nDistance++
}

//undoNullMove 撤消走一步空步
func (p *PositionStruct) undoNullMove() {
	p.nDistance--
	p.nMoveNum--
	p.changeSide()
}

//nullOkay 判断是否允许空步裁剪
func (p *PositionStruct) nullOkay() bool {
	if p.sdPlayer == 0 {
		return p.vlRed > NullMargin
	}
	return p.vlBlack > NullMargin
}

//generateMoves 生成所有走法，如果bCapture为true则只生成吃子走法
func (p *PositionStruct) generateMoves(mvs []int, bCapture bool) int {
	nGenMoves, pcSrc, sqDst, pcDst, nDelta := 0, 0, 0, 0, 0
	pcSelfSide := sideTag(p.sdPlayer)
	pcOppSide := oppSideTag(p.sdPlayer)

	for sqSrc := 0; sqSrc < 256; sqSrc++ {
		if !inBoard(sqSrc) {
			continue
		}

		//找到一个本方棋子，再做以下判断：
		pcSrc = p.ucpcSquares[sqSrc]
		if (pcSrc & pcSelfSide) == 0 {
			continue
		}

		//根据棋子确定走法
		switch pcSrc - pcSelfSide {
		case PieceJiang:
			for i := 0; i < 4; i++ {
				sqDst = sqSrc + ccJiangDelta[i]
				if !inFort(sqDst) {
					continue
				}
				pcDst = p.ucpcSquares[sqDst]
				if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
					mvs[nGenMoves] = move(sqSrc, sqDst)
					nGenMoves++
				}
			}
			break
		case PieceShi:
			for i := 0; i < 4; i++ {
				sqDst = sqSrc + ccShiDelta[i]
				if !inFort(sqDst) {
					continue
				}
				pcDst = p.ucpcSquares[sqDst]
				if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
					mvs[nGenMoves] = move(sqSrc, sqDst)
					nGenMoves++
				}
			}
			break
		case PieceXiang:
			for i := 0; i < 4; i++ {
				sqDst = sqSrc + ccShiDelta[i]
				if !(inBoard(sqDst) && noRiver(sqDst, p.sdPlayer) && p.ucpcSquares[sqDst] == 0) {
					continue
				}
				sqDst += ccShiDelta[i]
				pcDst = p.ucpcSquares[sqDst]
				if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
					mvs[nGenMoves] = move(sqSrc, sqDst)
					nGenMoves++
				}
			}
			break
		case PieceMa:
			for i := 0; i < 4; i++ {
				sqDst = sqSrc + ccJiangDelta[i]
				if p.ucpcSquares[sqDst] != 0 {
					continue
				}
				for j := 0; j < 2; j++ {
					sqDst = sqSrc + ccMaDelta[i][j]
					if !inBoard(sqDst) {
						continue
					}
					pcDst = p.ucpcSquares[sqDst]
					if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
						mvs[nGenMoves] = move(sqSrc, sqDst)
						nGenMoves++
					}
				}
			}
			break
		case PieceJu:
			for i := 0; i < 4; i++ {
				nDelta = ccJiangDelta[i]
				sqDst = sqSrc + nDelta
				for inBoard(sqDst) {
					pcDst = p.ucpcSquares[sqDst]
					if pcDst == 0 {
						if !bCapture {
							mvs[nGenMoves] = move(sqSrc, sqDst)
							nGenMoves++
						}
					} else {
						if (pcDst & pcOppSide) != 0 {
							mvs[nGenMoves] = move(sqSrc, sqDst)
							nGenMoves++
						}
						break
					}
					sqDst += nDelta
				}

			}
			break
		case PiecePao:
			for i := 0; i < 4; i++ {
				nDelta = ccJiangDelta[i]
				sqDst = sqSrc + nDelta
				for inBoard(sqDst) {
					pcDst = p.ucpcSquares[sqDst]
					if pcDst == 0 {
						if !bCapture {
							mvs[nGenMoves] = move(sqSrc, sqDst)
							nGenMoves++
						}
					} else {
						break
					}
					sqDst += nDelta
				}
				sqDst += nDelta
				for inBoard(sqDst) {
					pcDst = p.ucpcSquares[sqDst]
					if pcDst != 0 {
						if (pcDst & pcOppSide) != 0 {
							mvs[nGenMoves] = move(sqSrc, sqDst)
							nGenMoves++
						}
						break
					}
					sqDst += nDelta
				}
			}
			break
		case PieceBing:
			sqDst = squareForward(sqSrc, p.sdPlayer)
			if inBoard(sqDst) {
				pcDst = p.ucpcSquares[sqDst]
				if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
					mvs[nGenMoves] = move(sqSrc, sqDst)
					nGenMoves++
				}
			}
			if hasRiver(sqSrc, p.sdPlayer) {
				for nDelta = -1; nDelta <= 1; nDelta += 2 {
					sqDst = sqSrc + nDelta
					if inBoard(sqDst) {
						pcDst = p.ucpcSquares[sqDst]
						if (bCapture && (pcDst&pcOppSide) != 0) || (!bCapture && (pcDst&pcSelfSide) == 0) {
							mvs[nGenMoves] = move(sqSrc, sqDst)
							nGenMoves++
						}
					}
				}
			}
			break
		}
	}
	return nGenMoves
}

//legalMove 判断走法是否合理
func (p *PositionStruct) legalMove(mv int) bool {
	//判断起始格是否有自己的棋子
	sqSrc := src(mv)
	pcSrc := p.ucpcSquares[sqSrc]
	pcSelfSide := sideTag(p.sdPlayer)
	if (pcSrc & pcSelfSide) == 0 {
		return false
	}

	//判断目标格是否有自己的棋子
	sqDst := dst(mv)
	pcDst := p.ucpcSquares[sqDst]
	if (pcDst & pcSelfSide) != 0 {
		return false
	}

	//根据棋子的类型检查走法是否合理
	tmpPiece := pcSrc - pcSelfSide
	switch tmpPiece {
	case PieceJiang:
		return inFort(sqDst) && jiangSpan(sqSrc, sqDst)
	case PieceShi:
		return inFort(sqDst) && shiSpan(sqSrc, sqDst)
	case PieceXiang:
		return sameRiver(sqSrc, sqDst) && xiangSpan(sqSrc, sqDst) &&
			p.ucpcSquares[xiangPin(sqSrc, sqDst)] == 0
	case PieceMa:
		sqPin := maPin(sqSrc, sqDst)
		return sqPin != sqSrc && p.ucpcSquares[sqPin] == 0
	case PieceJu, PiecePao:
		nDelta := 0
		if sameX(sqSrc, sqDst) {
			if sqDst < sqSrc {
				nDelta = -1
			} else {
				nDelta = 1
			}
		} else if sameY(sqSrc, sqDst) {
			if sqDst < sqSrc {
				nDelta = -16
			} else {
				nDelta = 16
			}
		} else {
			return false
		}
		sqPin := sqSrc + nDelta
		for sqPin != sqDst && p.ucpcSquares[sqPin] == 0 {
			sqPin += nDelta
		}
		if sqPin == sqDst {
			return pcDst == 0 || tmpPiece == PieceJu
		} else if pcDst != 0 && tmpPiece == PiecePao {
			sqPin += nDelta
			for sqPin != sqDst && p.ucpcSquares[sqPin] == 0 {
				sqPin += nDelta
			}
			return sqPin == sqDst
		} else {
			return false
		}
	case PieceBing:
		if hasRiver(sqDst, p.sdPlayer) && (sqDst == sqSrc-1 || sqDst == sqSrc+1) {
			return true
		}
		return sqDst == squareForward(sqSrc, p.sdPlayer)
	default:

	}

	return false
}

//checked 判断是否被将军
func (p *PositionStruct) checked() bool {
	nDelta, sqDst, pcDst := 0, 0, 0
	pcSelfSide := sideTag(p.sdPlayer)
	pcOppSide := oppSideTag(p.sdPlayer)

	for sqSrc := 0; sqSrc < 256; sqSrc++ {
		//找到棋盘上的帅(将)，再做以下判断：
		if !inBoard(sqSrc) || p.ucpcSquares[sqSrc] != pcSelfSide+PieceJiang {
			continue
		}

		//判断是否被对方的兵(卒)将军
		if p.ucpcSquares[squareForward(sqSrc, p.sdPlayer)] == pcOppSide+PieceBing {
			return true
		}
		for nDelta = -1; nDelta <= 1; nDelta += 2 {
			if p.ucpcSquares[sqSrc+nDelta] == pcOppSide+PieceBing {
				return true
			}
		}

		//判断是否被对方的马将军(以仕(士)的步长当作马腿)
		for i := 0; i < 4; i++ {
			if p.ucpcSquares[sqSrc+ccShiDelta[i]] != 0 {
				continue
			}
			for j := 0; j < 2; j++ {
				pcDst = p.ucpcSquares[sqSrc+ccMaCheckDelta[i][j]]
				if pcDst == pcOppSide+PieceMa {
					return true
				}
			}
		}

		//判断是否被对方的车或炮将军(包括将帅对脸)
		for i := 0; i < 4; i++ {
			nDelta = ccJiangDelta[i]
			sqDst = sqSrc + nDelta
			for inBoard(sqDst) {
				pcDst = p.ucpcSquares[sqDst]
				if pcDst != 0 {
					if pcDst == pcOppSide+PieceJu || pcDst == pcOppSide+PieceJiang {
						return true
					}
					break
				}
				sqDst += nDelta
			}
			sqDst += nDelta
			for inBoard(sqDst) {
				pcDst = p.ucpcSquares[sqDst]
				if pcDst != 0 {
					if pcDst == pcOppSide+PiecePao {
						return true
					}
					break
				}
				sqDst += nDelta
			}
		}
		return false
	}
	return false
}

//isMate 判断是否被将死
func (p *PositionStruct) isMate() bool {
	pcCaptured := 0
	mvs := make([]int, MaxGenMoves)
	nGenMoveNum := p.generateMoves(mvs, false)
	for i := 0; i < nGenMoveNum; i++ {
		pcCaptured = p.movePiece(mvs[i])
		if !p.checked() {
			p.undoMovePiece(mvs[i], pcCaptured)
			return false
		}

		p.undoMovePiece(mvs[i], pcCaptured)
	}
	return true
}

//drawValue 和棋分值
func (p *PositionStruct) drawValue() int {
	if p.nDistance&1 == 0 {
		return -DrawValue
	}

	return DrawValue
}

//repStatus 检测重复局面
func (p *PositionStruct) repStatus(nRecur int) int {
	bSelfSide, bPerpCheck, bOppPerpCheck := false, true, true
	lpmvs := [MaxMoves]*MoveStruct{}
	for i := 0; i < MaxMoves; i++ {
		lpmvs[i] = p.mvsList[i]
	}

	for i := p.nMoveNum - 1; i >= 0 && lpmvs[i].wmv != 0 && lpmvs[i].ucpcCaptured == 0; i-- {
		if bSelfSide {
			bPerpCheck = bPerpCheck && lpmvs[i].ucbCheck
			if lpmvs[i].dwKey == p.zobr.dwKey {
				nRecur--
				if nRecur == 0 {
					result := 1
					if bPerpCheck {
						result += 2
					}
					if bOppPerpCheck {
						result += 4
					}
					return result
				}
			}
		} else {
			bOppPerpCheck = bOppPerpCheck && lpmvs[i].ucbCheck
		}
		bSelfSide = !bSelfSide
	}
	return 0
}

//repValue 重复局面分值
func (p *PositionStruct) repValue(nRepStatus int) int {
	vlReturn := 0
	if nRepStatus&2 != 0 {
		vlReturn += p.nDistance - BanValue
	}
	if nRepStatus&4 != 0 {
		vlReturn += BanValue - p.nDistance
	}

	if vlReturn == 0 {
		return p.drawValue()
	}

	return vlReturn
}

//mirror 对局面镜像
func (p *PositionStruct) mirror(posMirror *PositionStruct) {
	pc := 0
	posMirror.clearBoard()
	for sq := 0; sq < 256; sq++ {
		pc = p.ucpcSquares[sq]
		if pc != 0 {
			posMirror.addPiece(mirrorSquare(sq), pc)
		}
	}
	if p.sdPlayer == 1 {
		posMirror.changeSide()
	}
	posMirror.setIrrev()
}

//HashItem 置换表项结构
type HashItem struct {
	ucDepth   int    //深度
	ucFlag    int    //标志
	svl       int    //分值
	wmv       int    //最佳走法
	dwLock0   uint32 //校验码
	dwLock1   uint32 //校验码
	wReserved int    //保留
}

//BookItem 开局库项结构
type BookItem struct {
	dwLock uint32 //局面 Zobrist 校验码中的 dwLock1
	wmv    int    //走法
	wvl    int    //是权重(随机选择走法的几率，仅当两个相同的 dwLock 有不同的 wmv 时，wvl 的值才有意义)
}

//Search 与搜索有关的全局变量
type Search struct {
	mvResult      int                 //电脑走的棋
	nHistoryTable [65536]int          //历史表
	mvKillers     [LimitDepth][2]int  //杀手走法表
	hashTable     [HashSize]*HashItem //置换表
	BookTable     []*BookItem         //开局库
}

//searchBook 搜索开局库
func (p *PositionStruct) searchBook() int {
	bkToSearch := &BookItem{}
	mvs := make([]int, MaxGenMoves)
	vls := make([]int, MaxGenMoves)

	bookSize := len(p.search.BookTable)
	//如果没有开局库，则立即返回
	if bookSize <= 0 {
		return 0
	}

	//搜索当前局面
	bMirror := false
	bkToSearch.dwLock = p.zobr.dwLock1
	lpbk := sort.Search(bookSize, func(i int) bool {
		return p.search.BookTable[i].dwLock >= bkToSearch.dwLock
	})

	//如果没有找到，那么搜索当前局面的镜像局面
	if lpbk == bookSize || (lpbk < bookSize && p.search.BookTable[lpbk].dwLock != bkToSearch.dwLock) {
		bMirror = true
		posMirror := NewPositionStruct()
		p.mirror(posMirror)
		bkToSearch.dwLock = posMirror.zobr.dwLock1
		lpbk = sort.Search(bookSize, func(i int) bool {
			return p.search.BookTable[i].dwLock >= bkToSearch.dwLock
		})
	}
	//如果镜像局面也没找到，则立即返回
	if lpbk == bookSize || (lpbk < bookSize && p.search.BookTable[lpbk].dwLock != bkToSearch.dwLock) {
		return 0
	}
	//如果找到，则向前查第一个开局库项
	for lpbk >= 0 && p.search.BookTable[lpbk].dwLock == bkToSearch.dwLock {
		lpbk--
	}
	lpbk++
	//把走法和分值写入到"mvs"和"vls"数组中
	vl, nBookMoves, mv := 0, 0, 0
	for lpbk < bookSize && p.search.BookTable[lpbk].dwLock == bkToSearch.dwLock {
		if bMirror {
			mv = mirrorMove(p.search.BookTable[lpbk].wmv)
		} else {
			mv = p.search.BookTable[lpbk].wmv
		}
		if p.legalMove(mv) {
			mvs[nBookMoves] = mv
			vls[nBookMoves] = p.search.BookTable[lpbk].wvl
			vl += vls[nBookMoves]
			nBookMoves++
			if nBookMoves == MaxGenMoves {
				//防止"book.dat"中含有异常数据
				break
			}
		}
		lpbk++
	}
	if vl == 0 {
		//防止"book.dat"中含有异常数据
		return 0
	}
	//根据权重随机选择一个走法
	vl = rand.Intn(vl)
	i := 0
	for i = 0; i < nBookMoves; i++ {
		vl -= vls[i]
		if vl < 0 {
			break
		}
	}
	return mvs[i]
}

//probeHash 提取置换表项
func (p *PositionStruct) probeHash(vlAlpha, vlBeta, nDepth int) (int, int) {
	hsh := p.search.hashTable[p.zobr.dwKey&(HashSize-1)]
	if hsh.dwLock0 != p.zobr.dwLock0 || hsh.dwLock1 != p.zobr.dwLock1 {
		return -MateValue, 0
	}
	mv := hsh.wmv
	bMate := false
	if hsh.svl > WinValue {
		if hsh.svl < BanValue {
			//可能导致搜索的不稳定性，立刻退出，但最佳着法可能拿到
			return -MateValue, mv
		}
		hsh.svl -= p.nDistance
		bMate = true
	} else if hsh.svl < -WinValue {
		if hsh.svl > -BanValue {
			//同上
			return -MateValue, mv
		}
		hsh.svl += p.nDistance
		bMate = true
	}
	if hsh.ucDepth >= nDepth || bMate {
		if hsh.ucFlag == HashBeta {
			if hsh.svl >= vlBeta {
				return hsh.svl, mv
			}
			return -MateValue, mv
		} else if hsh.ucFlag == HashAlpha {
			if hsh.svl <= vlAlpha {
				return hsh.svl, mv
			}
			return -MateValue, mv
		}
		return hsh.svl, mv
	}
	return -MateValue, mv
}

//RecordHash 保存置换表项
func (p *PositionStruct) RecordHash(nFlag, vl, nDepth, mv int) {
	hsh := p.search.hashTable[p.zobr.dwKey&(HashSize-1)]
	if hsh.ucDepth > nDepth {
		return
	}
	hsh.ucFlag = nFlag
	hsh.ucDepth = nDepth
	if vl > WinValue {
		//可能导致搜索的不稳定性，并且没有最佳着法，立刻退出
		if mv == 0 && vl <= BanValue {
			return
		}
		hsh.svl = vl + p.nDistance
	} else if vl < -WinValue {
		if mv == 0 && vl >= -BanValue {
			return //同上
		}
		hsh.svl = vl - p.nDistance
	} else {
		hsh.svl = vl
	}
	hsh.wmv = mv
	hsh.dwLock0 = p.zobr.dwLock0
	hsh.dwLock1 = p.zobr.dwLock1
	p.search.hashTable[p.zobr.dwKey&(HashSize-1)] = hsh
}

//mvvLva 求MVV/LVA值
func (p *PositionStruct) mvvLva(mv int) int {
	return (cucMvvLva[p.ucpcSquares[dst(mv)]] << 3) - cucMvvLva[p.ucpcSquares[src(mv)]]
}

//SortStruct 走法排序结构
type SortStruct struct {
	mvHash    int   //置换表走法
	mvKiller1 int   //杀手走法
	mvKiller2 int   //杀手走法
	nPhase    int   //当前阶段
	nIndex    int   //当前采用第几个走法
	nGenMoves int   //总共有几个走法
	mvs       []int //所有的走法
}

//initSort 初始化，设定置换表走法和两个杀手走法
func (p *PositionStruct) initSort(mvHash int, s *SortStruct) {
	if s == nil {
		return
	}

	s.mvHash = mvHash
	s.mvKiller1 = p.search.mvKillers[p.nDistance][0]
	s.mvKiller2 = p.search.mvKillers[p.nDistance][1]
	s.nPhase = PhaseHash
}

//nextSort 得到下一个走法
func (p *PositionStruct) nextSort(s *SortStruct) int {
	if s == nil {
		return 0
	}

	switch s.nPhase {
	case PhaseHash:
		//置换表着法启发，完成后立即进入下一阶段；
		s.nPhase = PhaseKiller1
		if s.mvHash != 0 {
			return s.mvHash
		}
		fallthrough
	case PhaseKiller1:
		//杀手着法启发(第一个杀手着法)，完成后立即进入下一阶段；
		s.nPhase = PhaseKiller2
		if s.mvKiller1 != s.mvHash && s.mvKiller1 != 0 && p.legalMove(s.mvKiller1) {
			return s.mvKiller1
		}
		fallthrough
	case PhaseKiller2:
		//杀手着法启发(第二个杀手着法)，完成后立即进入下一阶段；
		s.nPhase = PhaseGenMoves
		if s.mvKiller2 != s.mvHash && s.mvKiller2 != 0 && p.legalMove(s.mvKiller2) {
			return s.mvKiller2
		}
		fallthrough
	case PhaseGenMoves:
		//生成所有着法，完成后立即进入下一阶段；
		s.nPhase = PhaseRest
		s.nGenMoves = p.generateMoves(s.mvs, false)
		s.mvs = s.mvs[:s.nGenMoves]
		sort.Slice(s.mvs, func(a, b int) bool {
			return p.search.nHistoryTable[a] > p.search.nHistoryTable[b]
		})
		s.nIndex = 0
		fallthrough
	case PhaseRest:
		//对剩余着法做历史表启发；
		for s.nIndex < s.nGenMoves {
			mv := s.mvs[s.nIndex]
			s.nIndex++
			if mv != s.mvHash && mv != s.mvKiller1 && mv != s.mvKiller2 {
				return mv
			}
		}
	default:
		//5. 没有着法了，返回零。
	}

	return 0
}

//setBestMove 对最佳走法的处理
func (p *PositionStruct) setBestMove(mv, nDepth int) {
	p.search.nHistoryTable[mv] += nDepth * nDepth
	if p.search.mvKillers[p.nDistance][0] != mv {
		p.search.mvKillers[p.nDistance][1] = p.search.mvKillers[p.nDistance][0]
		p.search.mvKillers[p.nDistance][0] = mv
	}
}

//searchQuiesc 静态(Quiescence)搜索过程
func (p *PositionStruct) searchQuiesc(vlAlpha, vlBeta int) int {
	nGenMoves := 0
	mvs := make([]int, MaxGenMoves)

	//检查重复局面
	vl := p.repStatus(1)
	if vl != 0 {
		return p.repValue(vl)
	}

	//到达极限深度就返回局面评价
	if p.nDistance == LimitDepth {
		return p.evaluate()
	}

	vlBest := -MateValue
	//这样可以知道，是否一个走法都没走过(杀棋)
	if p.inCheck() {
		//如果被将军，则生成全部走法
		nGenMoves = p.generateMoves(mvs, false)
		mvs = mvs[:nGenMoves]
		sort.Slice(mvs, func(a, b int) bool {
			return p.search.nHistoryTable[a] > p.search.nHistoryTable[b]
		})
	} else {
		//如果不被将军，先做局面评价
		vl = p.evaluate()
		if vl > vlBest {
			vlBest = vl
			if vl >= vlBeta {
				return vl
			}
			if vl > vlAlpha {
				vlAlpha = vl
			}
		}

		//如果局面评价没有截断，再生成吃子走法
		nGenMoves = p.generateMoves(mvs, true)
		mvs = mvs[:nGenMoves]
		sort.Slice(mvs, func(a, b int) bool {
			return p.mvvLva(mvs[a]) > p.mvvLva(mvs[b])
		})
	}

	//逐一走这些走法，并进行递归
	for i := 0; i < nGenMoves; i++ {
		if p.makeMove(mvs[i]) {
			vl = -p.searchQuiesc(-vlBeta, -vlAlpha)
			p.undoMakeMove()

			//进行Alpha-Beta大小判断和截断
			if vl > vlBest {
				//找到最佳值(但不能确定是Alpha、PV还是Beta走法)
				//"vlBest"就是目前要返回的最佳值，可能超出Alpha-Beta边界
				vlBest = vl
				//找到一个Beta走法
				if vl >= vlBeta {
					//Beta截断
					return vl
				}
				//找到一个PV走法
				if vl > vlAlpha {
					//缩小Alpha-Beta边界
					vlAlpha = vl
				}
			}
		}
	}

	//所有走法都搜索完了，返回最佳值
	if vlBest == -MateValue {
		return p.nDistance - MateValue
	}
	return vlBest
}

//searchFull 超出边界(Fail-Soft)的Alpha-Beta搜索过程
func (p *PositionStruct) searchFull(vlAlpha, vlBeta, nDepth int, bNoNull bool) int {
	vl, mvHash, nNewDepth := 0, 0, 0

	//到达水平线，则调用静态搜索(注意：由于空步裁剪，深度可能小于零)
	if nDepth <= 0 {
		return p.searchQuiesc(vlAlpha, vlBeta)
	}

	//检查重复局面(注意：不要在根节点检查，否则就没有走法了)
	vl = p.repStatus(1)
	if vl != 0 {
		return p.repValue(vl)
	}

	//到达极限深度就返回局面评价
	if p.nDistance == LimitDepth {
		return p.evaluate()
	}

	//尝试置换表裁剪，并得到置换表走法
	vl, mvHash = p.probeHash(vlAlpha, vlBeta, nDepth)
	if vl > -MateValue {
		return vl
	}

	//尝试空步裁剪(根节点的Beta值是"MateValue"，所以不可能发生空步裁剪)
	if !bNoNull && !p.inCheck() && p.nullOkay() {
		p.nullMove()
		vl = -p.searchFull(-vlBeta, 1-vlBeta, nDepth-NullDepth-1, true)
		p.undoNullMove()
		if vl >= vlBeta {
			return vl
		}
	}

	//初始化最佳值和最佳走法
	nHashFlag := HashAlpha
	//是否一个走法都没走过(杀棋)
	vlBest := -MateValue
	//是否搜索到了Beta走法或PV走法，以便保存到历史表
	mvBest := 0

	//初始化走法排序结构
	tmpSort := &SortStruct{
		mvs: make([]int, MaxGenMoves),
	}
	p.initSort(mvHash, tmpSort)

	//逐一走这些走法，并进行递归
	for mv := p.nextSort(tmpSort); mv != 0; mv = p.nextSort(tmpSort) {
		if p.makeMove(mv) {
			//将军延伸
			if p.inCheck() {
				nNewDepth = nDepth
			} else {
				nNewDepth = nDepth - 1
			}
			//PVS
			if vlBest == -MateValue {
				vl = -p.searchFull(-vlBeta, -vlAlpha, nNewDepth, false)
			} else {
				vl = -p.searchFull(-vlAlpha-1, -vlAlpha, nNewDepth, false)
				if vl > vlAlpha && vl < vlBeta {
					vl = -p.searchFull(-vlBeta, -vlAlpha, nNewDepth, false)
				}
			}
			p.undoMakeMove()

			//进行Alpha-Beta大小判断和截断
			if vl > vlBest {
				//找到最佳值(但不能确定是Alpha、PV还是Beta走法)
				vlBest = vl
				//vlBest就是目前要返回的最佳值，可能超出Alpha-Beta边界
				if vl >= vlBeta {
					//找到一个Beta走法, Beta走法要保存到历史表, 然后截断
					nHashFlag = HashBeta
					mvBest = mv
					break
				}
				if vl > vlAlpha {
					//找到一个PV走法，PV走法要保存到历史表，缩小Alpha-Beta边界
					nHashFlag = HashPV
					mvBest = mv
					vlAlpha = vl
				}
			}
		}
	}

	//所有走法都搜索完了，把最佳走法(不能是Alpha走法)保存到历史表，返回最佳值
	if vlBest == -MateValue {
		//如果是杀棋，就根据杀棋步数给出评价
		return p.nDistance - MateValue
	}
	//记录到置换表
	p.RecordHash(nHashFlag, vlBest, nDepth, mvBest)
	if mvBest != 0 {
		//如果不是Alpha走法，就将最佳走法保存到历史表
		p.setBestMove(mvBest, nDepth)
	}
	return vlBest
}

//searchRoot 根节点的Alpha-Beta搜索过程
func (p *PositionStruct) searchRoot(nDepth int) int {
	vl, nNewDepth := 0, 0
	vlBest := -MateValue

	//初始化走法排序结构
	tmpSort := &SortStruct{
		mvs: make([]int, MaxGenMoves),
	}
	p.initSort(p.search.mvResult, tmpSort)

	//逐一走这些走法，并进行递归
	for mv := p.nextSort(tmpSort); mv != 0; mv = p.nextSort(tmpSort) {
		if p.makeMove(mv) {
			if p.inCheck() {
				nNewDepth = nDepth
			} else {
				nNewDepth = nDepth - 1
			}
			if vlBest == -MateValue {
				vl = -p.searchFull(-MateValue, MateValue, nNewDepth, true)
			} else {
				vl = -p.searchFull(-vlBest-1, -vlBest, nNewDepth, false)
				if vl > vlBest {
					vl = -p.searchFull(-MateValue, -vlBest, nNewDepth, true)
				}
			}
			p.undoMakeMove()
			if vl > vlBest {
				vlBest = vl
				p.search.mvResult = mv
				if vlBest > -WinValue && vlBest < WinValue {
					vlBest += int(rand.Int31()&RandomMask) - int(rand.Int31()&RandomMask)
				}
			}
		}
	}
	p.RecordHash(HashPV, vlBest, nDepth, p.search.mvResult)
	p.setBestMove(p.search.mvResult, nDepth)
	return vlBest
}

//searchMain 迭代加深搜索过程
func (p *PositionStruct) searchMain() {
	//清空历史表
	for i := 0; i < 65536; i++ {
		p.search.nHistoryTable[i] = 0
	}
	//清空杀手走法表
	for i := 0; i < LimitDepth; i++ {
		for j := 0; j < 2; j++ {
			p.search.mvKillers[i][j] = 0
		}
	}
	//清空置换表
	for i := 0; i < HashSize; i++ {
		p.search.hashTable[i].ucDepth = 0
		p.search.hashTable[i].ucFlag = 0
		p.search.hashTable[i].svl = 0
		p.search.hashTable[i].wmv = 0
		p.search.hashTable[i].wReserved = 0
		p.search.hashTable[i].dwLock0 = 0
		p.search.hashTable[i].dwLock1 = 0
	}
	//初始化定时器
	start := time.Now()
	//初始步数
	p.nDistance = 0

	//搜索开局库
	p.search.mvResult = p.searchBook()
	if p.search.mvResult != 0 {
		p.makeMove(p.search.mvResult)
		if p.repStatus(3) == 0 {
			p.undoMakeMove()
			return
		}
		p.undoMakeMove()
	}

	//检查是否只有唯一走法
	vl := 0
	mvs := make([]int, MaxGenMoves)
	nGenMoves := p.generateMoves(mvs, false)
	for i := 0; i < nGenMoves; i++ {
		if p.makeMove(mvs[i]) {
			p.undoMakeMove()
			p.search.mvResult = mvs[i]
			vl++
		}
	}
	if vl == 1 {
		return
	}

	//迭代加深过程
	rand.Seed(time.Now().UnixNano())
	for i := 1; i <= LimitDepth; i++ {
		vl = p.searchRoot(i)
		//搜索到杀棋，就终止搜索
		if vl > WinValue || vl < -WinValue {
			break
		}
		//超过一秒，就终止搜索
		if time.Now().Sub(start).Milliseconds() > 1000 {
			break
		}
	}
}

//printBoard 打印棋盘
func (p *PositionStruct) printBoard() {
	stdString := "\n"
	for i, v := range p.ucpcSquares {
		if (i+1)%16 == 0 {
			tmpString := fmt.Sprintf("%2d\n", v)
			stdString += tmpString
		} else {
			tmpString := fmt.Sprintf("%2d ", v)
			stdString += tmpString
		}
	}
	fmt.Print(stdString)
}
