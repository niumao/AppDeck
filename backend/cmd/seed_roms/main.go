package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
)

type romEntry struct {
	CN string // Chinese display name
	EN string // English filename (without extension)
}

// FC (NES)
var fcROMs = []romEntry{
	{"F1赛车", "F-1 Race (Japan)"},
	{"三国志2-霸王的大陆", "Sangokushi II - Haou no Tairiku (Japan)"},
	{"三目童子", "Mitsume ga Tooru (Japan)"},
	{"中东战争", "Conflict (Japan)"},
	{"中华大仙", "Chuuka Taisen (Japan)"},
	{"人间兵器", "Section Z (Japan)"},
	{"兔宝宝历险记1", "Tiny Toon Adventures (Japan)"},
	{"兵峰", "TwinBee (Japan)"},
	{"兵峰原版", "TwinBee (Japan) (Original)"},
	{"冒险岛1", "Takahashi Meijin no Bouken Jima (Japan)"},
	{"冒险岛2", "Takahashi Meijin no Bouken Jima II (Japan)"},
	{"冒险岛3", "Takahashi Meijin no Bouken Jima III (Japan)"},
	{"冒险岛4", "Takahashi Meijin no Bouken Jima IV (Japan)"},
	{"功夫", "Yie Ar Kung-Fu (Japan)"},
	{"加纳战机", "Gun-Nac (Japan)"},
	{"勇者斗恶龙6", "Dragon Quest VI (Hack)"},
	{"双截龙1", "Double Dragon (Japan)"},
	{"双截龙2", "Double Dragon II - The Revenge (Japan)"},
	{"双截龙3", "Double Dragon III - The Rosetta Stone (Japan)"},
	{"古巴战士", "Guerrilla War (USA)"},
	{"吃豆人", "Pac-Man (Japan) (Namco)"},
	{"吃豆人原版", "Pac-Man (Japan) (Namco) (Original)"},
	{"合金装备2", "Snake's Revenge (USA)"},
	{"吞食天地2", "Tenchi wo Kurau II - Shokatsu Koumei Den (Japan)"},
	{"圣灵传说", "Seirei Densetsu (Japan)"},
	{"地下城与勇士", "Dungeon & Warriors (Hack)"},
	{"坦克大战", "Battle City (Japan)"},
	{"坦克大战原版", "Battle City (Japan) (Original)"},
	{"塞尔达传说", "Zelda no Densetsu 1 - The Hyrule Fantasy (Japan) (Rev 1)"},
	{"塞尔达传说原版", "Zelda no Densetsu 1 - The Hyrule Fantasy (Japan) (Original)"},
	{"大金刚1", "Donkey Kong (World) (Rev 1)"},
	{"大金刚2", "Donkey Kong Jr. (World)"},
	{"大金刚3", "Donkey Kong 3 (Japan)"},
	{"大金刚JR", "Donkey Kong Jr. (World) (Alt)"},
	{"大金刚JR原版", "Donkey Kong Jr. (World) (Original)"},
	{"天才厨师", "BurgerTime (Japan)"},
	{"奥瓦的觉醒", "Awa no Mezame (Hack)"},
	{"封神榜", "Houshin Engi (Japan)"},
	{"影子传说", "Kage no Densetsu - The Legend of Kage (Japan)"},
	{"微型法师", "Micro Mages (Homebrew)"},
	{"忍者神龟1", "Teenage Mutant Ninja Turtles (Japan)"},
	{"忍者神龟2", "Teenage Mutant Ninja Turtles II - The Arcade Game (Japan)"},
	{"忍者龙剑传1", "Ninja Ryuukenden (Japan)"},
	{"忍者龙剑传1原版", "Ninja Ryuukenden (Japan) (Original)"},
	{"忍者龙剑传2", "Ninja Ryuukenden II - Ankoku no Jashin Ken (Japan)"},
	{"忍者龙剑传2原版", "Ninja Ryuukenden II - Ankoku no Jashin Ken (Japan) (Original)"},
	{"忍者龙剑传3", "Ninja Ryuukenden III - Yomi no Hakobune (Japan)"},
	{"快打旋风", "Mighty Final Fight (Japan)"},
	{"恶魔城16bit", "Castlevania 16bit (Hack)"},
	{"恶魔城1", "Akumajou Dracula (Japan)"},
	{"恶魔城3", "Akumajou Densetsu (Japan)"},
	{"恶魔城MSX", "Castlevania MSX (Hack)"},
	{"战地小子", "Ikari (Japan)"},
	{"战地小子2", "Ikari II - Dogosoken (Japan)"},
	{"敲冰块", "Ice Climber (Japan)"},
	{"敲冰块原版", "Ice Climber (Japan) (Original)"},
	{"星之卡比 梦之泉物语", "Hoshi no Kirby - Yume no Izumi no Monogatari (Japan)"},
	{"松鼠大作战1", "Chip to Dale no Daisakusen (Japan)"},
	{"松鼠大作战2", "Chip to Dale no Daisakusen 2 (Japan)"},
	{"梦之勇士", "Yume no Yuusha (Japan)"},
	{"气球大战", "Balloon Fight (Japan)"},
	{"气球大战高清版", "Balloon Fight HD (Hack)"},
	{"水上魂斗罗", "Super Contra (Japan)"},
	{"沙罗曼蛇", "Salamander (Japan)"},
	{"洛克人", "Rockman (Japan)"},
	{"洛克人原版", "Rockman (Japan) (Original)"},
	{"火炮侦探", "Cannon Detective (Hack)"},
	{"火箭车", "Road Fighter (Japan)"},
	{"炸弹人", "Bomberman (Japan)"},
	{"热血冰球", "Ike Ike! Nekketsu Hockey Bu - Subette Koronde Dairantou (Japan)"},
	{"热血新纪录", "Downtown Nekketsu Koushinkyoku - Soreyuke Daiundoukai (Japan)"},
	{"热血格斗传说", "Nekketsu Kakutou Densetsu (Japan)"},
	{"热血物语", "Downtown Nekketsu Monogatari (Japan)"},
	{"热血篮球", "Nekketsu Street Basket - Ganbare Dunk Heroes (Japan)"},
	{"热血足球", "Nekketsu Koukou Dodgeball Bu - Soccer Hen (Japan)"},
	{"热血进行曲", "Downtown Nekketsu March - Super-Awesome Field Day! (Japan)"},
	{"燃烧游侠", "Moero TwinBee - Cinnamon-hakase wo Sukue! (Japan)"},
	{"爱的小屋", "Ai no Koya (Hack)"},
	{"爱的小屋原版", "Ai no Koya (Hack) (Original)"},
	{"特救指令", "S.C.A.T. - Special Cybernetic Attack Team (USA)"},
	{"猪小弟", "Buta Otouto (Hack)"},
	{"白烂贱客", "Jay and Silent Bob (Hack)"},
	{"神龟激斗", "Teenage Mutant Ninja Turtles - Tournament Fighters (Japan)"},
	{"空中魂斗罗", "Contra (Japan)"},
	{"绿色兵团", "Green Beret (Japan)"},
	{"脱狱1", "P.O.W. - Prisoners of War (USA)"},
	{"脱狱2", "P.O.W. - Prisoners of War 2 (Hack)"},
	{"花式九球", "Side Pocket (Japan)"},
	{"荒野大镖客", "Gun.Smoke (Japan)"},
	{"赤影战士", "Kage - Shadow of the Ninja (Japan)"},
	{"赤色要塞", "Jackal (Japan)"},
	{"超时空要塞", "Choujikuu Yousai - Macross (Japan)"},
	{"超级马里奥兄弟", "Super Mario Bros. (World)"},
	{"超级马里奥兄弟2", "Super Mario Bros. 2 (USA)"},
	{"超级马里奥兄弟3d", "Super Mario Bros. 3D (Hack)"},
	{"超级马里奥兄弟flash", "Super Mario Bros. Flash (Hack)"},
	{"超级马里奥兄弟paper", "Super Mario Bros. Paper (Hack)"},
	{"重装机兵", "Metal Max (Japan)"},
	{"银河战士", "Metroid (Japan)"},
	{"银河战士原版", "Metroid (Japan) (Original)"},
	{"闪亮亮星星之夜DX", "Starry Night DX (Hack)"},
	{"雪人兄弟", "Snow Bros. (Japan)"},
	{"马戏团", "Circus Charlie (Japan)"},
	{"马里奥医生", "Dr. Mario (World)"},
	{"高速公路赛车", "Road Fighter (Japan) (v1)"},
	{"高速公路赛车II", "Road Fighter II (Japan)"},
	{"魂斗罗2", "Super Contra (Japan) (v2)"},
	{"魂斗罗3 力量", "Contra Force (USA)"},
	{"魂斗罗日版", "Contra (Japan) (v1)"},
}

// SFC (SNES)
var sfcROMs = []romEntry{
	{"46亿年物语", "46 Okunen Monogatari - Harukanaru Eden e (Japan)"},
	{"GO!GO!恶魔少年", "Go Go Ackman (Japan)"},
	{"Q版沙罗曼蛇", "Parodius Da! - Shinwa kara Owarai e (Japan)"},
	{"SD英雄大战 2", "The Great Battle II - Last Fighter Twin (Japan)"},
	{"SD英雄大战 3", "The Great Battle III (Japan)"},
	{"SD英雄大战 4", "The Great Battle IV (Japan)"},
	{"SD英雄大战 5", "The Great Battle V (Japan)"},
	{"SD英雄大战外传 2", "SD The Great Battle - Gaiden 2 (Japan)"},
	{"Super Mario RPG (C) (V1.0)", "Super Mario RPG (Japan)"},
	{"yybszzp", "yybszzp (Hack)"},
	{"ն3", "Unknown 3 (Hack)"},
	{"三国志3", "Sangokushi III (Japan)"},
	{"三国志英杰传", "Sangokushi Eiketsuden (Japan)"},
	{"不给力的雷斯特", "Lester the Unlikely (USA)"},
	{"伊苏3", "Ys III - Wanderers from Ys (Japan)"},
	{"伊苏4", "Ys IV - Mask of the Sun (Japan)"},
	{"传说战士", "Legend Warriors (Hack)"},
	{"俄罗斯方块与马里奥医生", "Tetris & Dr. Mario (USA)"},
	{"修罗战士 - 钢", "Shura no Mon (Japan)"},
	{"僵尸邻居", "Zombies Ate My Neighbors (USA)"},
	{"兵蜂", "Pop'n TwinBee (Japan)"},
	{"兵蜂 - 彩虹铃当的冒险", "Pop'n TwinBee - Rainbow Bell Adventures (Europe)"},
	{"冲！冲！恶魔少年 2", "Go Go Ackman 2 (Japan)"},
	{"冲！冲！恶魔少年 3", "Go Go Ackman 3 (Japan)"},
	{"刚果小子", "Congo's Caper (USA)"},
	{"初代热血硬派", "Shin Nekketsu Kouha - Kunio-tachi no Banka (Japan)"},
	{"前线任务", "Front Mission (Japan)"},
	{"勇者斗恶龙3", "Dragon Quest III - Soshite Densetsu e... (Japan)"},
	{"卡比梦幻岛 3", "Hoshi no Kirby 3 (Japan)"},
	{"卡比超级之星", "Hoshi no Kirby - Super Deluxe (Japan)"},
	{"吸血鬼伯爵", "Nosferatu (Japan)"},
	{"哆啦A梦4 - 大雄和月之王国", "Doraemon 4 - Nobita to Tuki no Oukoku (Japan)"},
	{"噩梦破坏者", "Nightmare Busters (Prototype)"},
	{"圣剑传说3", "Seiken Densetsu 3 (Japan)"},
	{"地球防卫军", "Earth Defense Force (USA)"},
	{"塞尔达传说", "Zelda no Densetsu - Kamigami no Triforce (Japan)"},
	{"大力工头源", "Ganbare Daiku no Gensan (Japan)"},
	{"大盗伍佑卫门", "Ganbare Goemon - Yukihime Kyuushutsu Emaki (Japan)"},
	{"大盗伍佑卫门 2", "Ganbare Goemon 2 - Kiteretsu Shougun Magginesu (Japan)"},
	{"大盗伍佑卫门 3", "Ganbare Goemon 3 - Shishijyuurokubei no Karakuri Manji Gatame (Japan)"},
	{"大盗伍佑卫门 4", "Ganbare Goemon 4 - Kirakira Douchuu - Boku ga Dancer ni Natta Wake (Japan)"},
	{"天地创造", "Tenchi Souzou (Japan)"},
	{"天外魔境", "Tengai Makyou Zero (Japan)"},
	{"天龙斩", "Tenryuu Zan (Japan)"},
	{"失落的维京人", "The Lost Vikings (USA)"},
	{"失落的维京人 2", "The Lost Vikings II (USA)"},
	{"奇奇怪界 - 月夜草子", "KiKi KaiKai - Tsukiyo Soushi (Japan)"},
	{"奇奇怪界 - 神秘的黑衣男子", "KiKi KaiKai - Nazo no Kuro Manto (Japan)"},
	{"宇宙巡航机 3", "Gradius III (Japan)"},
	{"实况极上Q版沙罗曼蛇", "Jikkyou Oshaberi Parodius (Japan)"},
	{"少年忍者佐助", "Shounen Ninja Sasuke (Japan)"},
	{"幽游白书 2 - 格斗之章", "Yuu Yuu Hakusho 2 (Japan)"},
	{"幽游白书终篇 - 魔界最强列传", "Yuu Yuu Hakusho Final - Makai Saikyou Retsuden (Japan)"},
	{"忍者战士", "The Ninja Warriors Again (Japan)"},
	{"忍者神龟 4", "Teenage Mutant Ninja Turtles IV - Turtles in Time (Japan)"},
	{"忍者蛙", "Battletoads in Battlemaniacs (Japan)"},
	{"忍者蛙与双截龙", "Battletoads & Double Dragon - The Ultimate Team (USA)"},
	{"忍者龙剑传 巴", "Ninja Ryuukenden Tomoe (Japan)"},
	{"快打刑事", "Rushing Beat (Japan)"},
	{"快打刑事 - 乱", "Rushing Beat Ran - Fukusei Toshi (Japan)"},
	{"快打刑事 - 修罗", "Rushing Beat Shura (Japan)"},
	{"快打旋风", "Final Fight (Japan)"},
	{"快打旋风 2", "Final Fight 2 (Japan)"},
	{"快打旋风 3", "Final Fight 3 (USA)"},
	{"快打旋风 凯", "Final Fight Guy (Japan)"},
	{"恶魔城 XX", "Akumajou Dracula XX (Japan)"},
	{"战斗潜水艇", "Sentou Sensuikan (Japan)"},
	{"摩登原始人 - 马德罗克山的宝藏", "The Flintstones - The Treasure of Sierra Madrock (USA)"},
	{"斗姬传", "Toukiden (Japan)"},
	{"时空勇士", "Jikuu Yuushi (Japan)"},
	{"星际火狐", "Star Fox (USA)"},
	{"暗黑的破坏神", "Diablo (Hack)"},
	{"暴力刑警", "Violent Storm (Hack)"},
	{"极上Q版沙罗曼蛇", "Gokujou Parodius - Kako no Eikoh wo Motomete (Japan)"},
	{"梦想家", "Dreamer (Hack)"},
	{"棉花小魔女", "Magical Pop'n (Japan)"},
	{"横山光辉三国志", "Yokoyama Mitsuteru Sangokushi (Japan)"},
	{"毁灭骑兵", "Rendering Ranger R2 (Japan)"},
	{"波斯王子", "Prince of Persia (Japan)"},
	{"波斯王子 2", "Prince of Persia 2 - The Shadow & The Flame (USA)"},
	{"洛克人 7", "Rockman 7 - Shukumei no Taiketsu! (Japan)"},
	{"洛克人 X", "Rockman X (Japan)"},
	{"洛克人 X2", "Rockman X2 (Japan)"},
	{"洛克人 X3", "Rockman X3 (Japan)"},
	{"洛克人与福尔特", "Rockman & Forte (Japan)"},
	{"浪漫沙加3", "Romancing SaGa 3 (Japan)"},
	{"灌篮高手2", "From TV Animation Slam Dunk 2 (Japan)"},
	{"火焰之纹章3 - 纹章之谜", "Fire Emblem - Monshou no Nazo (Japan)"},
	{"火焰之纹章4 - 圣战系谱", "Fire Emblem - Seisen no Keifu (Japan)"},
	{"火焰之纹章5 - 多拉基亚776", "Fire Emblem - Thracia 776 (Japan)"},
	{"灭火英雄", "The Firemen (Japan)"},
	{"热血硬派 - 国雄之挽歌", "Shin Nekketsu Kouha - Kunio-tachi no Banka (Japan) (v2)"},
	{"热血高校躲避球全员集合", "Nekketsu Koukou Dodgeball Bu (Japan)"},
	{"特技立体赛车", "Wild Trax (Japan)"},
	{"狮子王", "The Lion King (USA)"},
	{"用此系统及游戏获利者死全家184", "Pirate Warning 184 (Hack)"},
	{"电精", "Psycho Dream (Japan)"},
	{"盖亚幻想纪", "Gaia Gensouki (Japan)"},
	{"秀逗泰山", "George of the Jungle (Hack)"},
	{"第3次超级机器人大战", "Dai-3-ji Super Robot Taisen (Japan)"},
	{"米奇狂热 - 米老鼠的永恒冒险", "Mickey Mania - The Timeless Adventures of Mickey Mouse (USA)"},
	{"绀碧舰队", "Konpeki no Kantai (Japan)"},
	{"美食战队", "Gourmet Sentai (Japan)"},
	{"荒野双枪", "Wild Guns (Japan)"},
	{"蝙蝠侠与罗宾的大冒险", "Adventures of Batman & Robin, The (USA)"},
	{"西部英豪-枪手的考验", "Sunset Riders (USA)"},
	{"财宝猎人G", "Treasure Hunter G (Japan)"},
	{"超人 - 死里逢生", "The Death and Return of Superman (USA)"},
	{"超兄贵 - 爆裂乱斗篇", "Chou Aniki - Bakuretsu Rantou Hen (Japan)"},
	{"超新星战机", "Super Nova (USA)"},
	{"超时空要塞", "Choujikuu Yousai Macross - Scrambled Valkyrie (Japan)"},
	{"超级冒险岛", "Super Adventure Island (USA)"},
	{"超级冒险岛 2", "Super Adventure Island II (USA)"},
	{"超级原人", "Chou Genjin (Japan)"},
	{"超级原人 2", "Chou Genjin 2 (Japan)"},
	{"超级大金刚", "Super Donkey Kong (Japan)"},
	{"超级大金刚 2", "Super Donkey Kong 2 - Dixie & Diddy (Japan)"},
	{"超级大金刚 3", "Super Donkey Kong 3 - Nazo no Krems Shima (Japan)"},
	{"超级异形战机", "Super R-Type (Japan)"},
	{"超级忍者君", "Super Ninja-kun (Japan)"},
	{"超级恶魔城 4", "Akumajou Dracula (Japan) (SFC)"},
	{"超级战士", "Super Turrican (USA)"},
	{"超级战士 2", "Super Turrican 2 (USA)"},
	{"超级星球大战 - 绝地归来", "Super Star Wars - Return of the Jedi (USA)"},
	{"超级炸弹人", "Super Bomberman (Japan)"},
	{"超级炸弹人 2", "Super Bomberman 2 (Japan)"},
	{"超级炸弹人 3", "Super Bomberman 3 (Japan)"},
	{"超级炸弹人 4", "Super Bomberman 4 (Japan)"},
	{"超级炸弹人 5", "Super Bomberman 5 (Japan)"},
	{"超级银河战士", "Super Metroid (Japan)"},
	{"超级马里奥世界", "Super Mario World (Japan)"},
	{"超级马里奥世界 2 - 耀西岛", "Super Mario World 2 - Yoshi's Island (USA)"},
	{"超级马里奥全明星 + 超级马里奥世界", "Super Mario All-Stars + Super Mario World (USA)"},
	{"超级马里奥赛车", "Super Mario Kart (Japan)"},
	{"超钢战机", "Chou Koutetsu Senki (Japan)"},
	{"超魔界村", "Chou Makaimura (Japan)"},
	{"迦楼罗王", "Karuraou (Japan)"},
	{"逃离异世界", "Isekai Escape (Hack)"},
	{"邪鬼破坏弹珠台", "Jaki Crush (Japan)"},
	{"钢铁战士", "Iron Commando (Japan)"},
	{"银河战国群雄传", "Ginga Sengoku Gun Yuuden - Rai (Japan)"},
	{"银河风暴", "Galaxy Storm (Hack)"},
	{"闪回", "Flashback (Japan)"},
	{"阿拉丁", "Aladdin (USA)"},
	{"陷阱 - 玛雅人的冒险", "Pitfall - The Mayan Adventure (USA)"},
	{"零式赛车", "F-Zero (Japan)"},
	{"音速超人 2", "Sonic Blastman II (Japan)"},
	{"飞行英雄", "Flying Hero (Japan)"},
	{"马里奥 月球探险", "Mario Moon Adventure (Hack)"},
	{"魂斗罗 3 - 异形大战", "Contra Spirits (Japan)"},
	{"魔兽王", "Majuu Ou (Japan)"},
	{"魔法剑 - 英雄的幻想", "Magic Sword (Japan)"},
	{"魔界村外传 - 纹章篇", "Makaimura - Monshou Hen (Hack)"},
	{"黄龙之耳", "Kouryuu no Mimi (Japan)"},
	{"黑水海盗", "Waterworld (USA)"},
	{"黑色荆棘", "Blackthorne (USA)"},
	{"龙珠 Z - 超武斗传 3", "Dragon Ball Z - Super Butouden 3 (Japan)"},
}

type romDef struct {
	Dir      string
	Core     string
	ThumbSys string
	Entries  []romEntry
}

func main() {
	cfg := config.Load()
	if err := database.InitMySQL(&cfg.Database); err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer database.Close()

	romDir := "/opt/EXP/appdeck2/roms"

	defs := []romDef{
		{Dir: "FC", Core: "nestopia", ThumbSys: "Nintendo - Nintendo Entertainment System", Entries: fcROMs},
		{Dir: "SFC", Core: "snes9x", ThumbSys: "Nintendo - Super Nintendo Entertainment System", Entries: sfcROMs},
	}

	// ✅ 全局唯一分类：街机（所有游戏都只属于这一个分类）
	const categoryName = "街机"
	catID, err := ensureSingleCategory(categoryName)
	if err != nil {
		log.Fatalf("创建分类失败: %v", err)
	}
	log.Printf("分类【街机】准备完成 (id=%s)", catID)

	// 遍历处理所有游戏
	for _, def := range defs {
		dirPath := filepath.Join(romDir, def.Dir)

		// 读取目录文件后缀映射（支持中文和英文文件名）
		fileExts := map[string]string{}
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			log.Fatalf("读取目录失败 %s: %v", dirPath, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			base := strings.TrimSuffix(e.Name(), ext)
			if ext == ".nes" || ext == ".sfc" || ext == ".smc" {
				fileExts[base] = ext
			}
		}

		updated, created := 0, 0

		for i, rom := range def.Entries {
			// 优先查找中文文件名，回退到英文文件名
			var ext string
			var romFileName string
			var found bool
			if ext, found = fileExts[rom.CN]; found {
				romFileName = rom.CN
			} else if ext, found = fileExts[rom.EN]; found {
				romFileName = rom.EN
			} else {
				log.Printf("  未找到游戏文件: %s / %s", rom.CN, rom.EN)
				continue
			}

			// 游戏路径（使用实际文件名，与设备一致）
			romPath := def.Dir + "/" + romFileName + ext

			// Icons served from local RetroArch thumbnails via playlist lookup
			// (no external URL needed — Android WebView resolves retroarch:// protocol)

			// 应用数据
			appData := &models.App{
				Name:          rom.CN,
				Description:   "街机 - " + rom.CN,
				LaunchType:    "retroarch",
				RetroarchCore: def.Core,
				RetroarchRom:  romPath,
				LaunchParam:   rom.CN,
				OrderNum:      388,
				UpdatedAt:     time.Now(),
			}

			// Upsert: match by CN name (stable key) or retroarch_rom path
			var existingApp models.App
			err := database.DB.Where("name = ? AND launch_type = 'retroarch'", rom.CN).First(&existingApp).Error
			if err != nil {
				// Also try matching by old EN-based romPath
				oldRomPath := def.Dir + "/" + rom.EN + ext
				err = database.DB.Where("retroarch_rom = ?", oldRomPath).First(&existingApp).Error
			}
			if err == nil {
				// 存在 → 更新
				appData.ID = existingApp.ID
				appData.CreatedAt = existingApp.CreatedAt
				database.DB.Save(appData)
				bindAppToSingleCategory(existingApp.ID, catID)
				updated++
			} else {
				// 不存在 → 创建
				appData.ID = uuid.New().String()
				appData.CreatedAt = time.Now()
				database.DB.Create(appData)
				bindAppToSingleCategory(appData.ID, catID)
				created++
			}
		}

		log.Printf("[%s] 处理完成：更新 %d 个，新增 %d 个", def.Dir, updated, created)
	}

	// 更新版本号
	database.DB.Exec("UPDATE versions SET version = version + 1 WHERE id = (SELECT id FROM versions LIMIT 1)")

	fmt.Println("✅ 全部执行完成！")
}

// ensureSingleCategory 创建/获取唯一分类：街机
func ensureSingleCategory(name string) (string, error) {
	var cat models.Category
	if err := database.DB.Where("name = ?", name).First(&cat).Error; err == nil {
		return cat.ID, nil
	}

	cat = models.Category{
		ID:       uuid.New().String(),
		Name:     name,
		OrderNum: 1,
	}
	if err := database.DB.Create(&cat).Error; err != nil {
		return "", err
	}
	return cat.ID, nil
}

// bindAppToSingleCategory 强制 App 只属于【街机】一个分类（先清空旧分类，再绑定新分类）
func bindAppToSingleCategory(appID, catID string) {
	// 先删除该 App 所有旧分类
	database.DB.Exec("DELETE FROM app_categories WHERE app_id = ?", appID)
	// 再唯一绑定街机分类
	database.DB.Exec("INSERT INTO app_categories (app_id, category_id) VALUES (?, ?)", appID, catID)
}